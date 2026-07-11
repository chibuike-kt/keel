-- Double-entry ledger schema. The invariants below are enforced here, in
-- Postgres, not just in the Go client — application code is the fast,
-- friendly path; these triggers are what actually makes the invariants
-- unbreakable, including against a future bug, a manual psql session, or
-- any other client that isn't this module's Go code.
--
-- Custom SQLSTATE codes (the "LGxxx" values RAISE EXCEPTION assigns below)
-- let the Go client distinguish which invariant fired without parsing
-- error message text. They're in Postgres's user-defined range and never
-- collide with a built-in error code.

CREATE TABLE accounts (
    id              BIGSERIAL PRIMARY KEY,
    external_ref    TEXT NOT NULL,
    currency        CHAR(3) NOT NULL,
    -- Cached projection of SUM(entries.amount) for this account, never an
    -- independent source of truth. VerifyBalance recomputes the sum
    -- independently so this projection can always be checked, not just
    -- trusted.
    balance         BIGINT NOT NULL DEFAULT 0,
    allow_overdraft BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT accounts_external_ref_key UNIQUE (external_ref)
);

CREATE TABLE transactions (
    id          BIGSERIAL PRIMARY KEY,
    -- Caller-supplied idempotency key: retrying a Post with the same
    -- reference must fail loudly (ErrDuplicateReference), not double-post.
    reference   TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transactions_reference_key UNIQUE (reference)
);

CREATE TABLE entries (
    id             BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    account_id     BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    -- Signed minor units: negative = debit, positive = credit. A zero-amount
    -- leg isn't a real entry, so it's rejected outright.
    amount         BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT entries_amount_nonzero_check CHECK (amount <> 0)
);

-- external_ref and reference already have a unique index from their UNIQUE
-- constraints above. transaction_id and account_id are plain foreign keys,
-- which Postgres does not index automatically, and both the balance
-- projection queries and the deferred constraint triggers below scan by
-- them constantly.
CREATE INDEX entries_transaction_id_idx ON entries (transaction_id);
CREATE INDEX entries_account_id_idx ON entries (account_id);

-- Invariant 1: append-only.
--
-- Financial history is immutable. A correction is a new, offsetting
-- transaction, never an edit to one that already happened — an UPDATE or
-- DELETE here would let history quietly diverge from whatever was already
-- reported, reconciled against, or relied on. This can't be a Go-level
-- rule: it has to hold even against a bug in a future version of this
-- client, a one-off psql session, or a different service that gets given
-- write access to this database later.
CREATE OR REPLACE FUNCTION ledger_forbid_update_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION USING
        ERRCODE = 'LG001',
        MESSAGE = format('ledger: %s is append-only; %s is not allowed', TG_TABLE_NAME, TG_OP);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entries_append_only
    BEFORE UPDATE OR DELETE ON entries
    FOR EACH ROW
    EXECUTE FUNCTION ledger_forbid_update_delete();

CREATE TRIGGER transactions_append_only
    BEFORE UPDATE OR DELETE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION ledger_forbid_update_delete();

-- Invariant 2: every transaction's entries sum to zero.
--
-- This is deferred to COMMIT rather than checked immediately: a
-- transaction's entries are inserted as separate rows, so the invariant
-- genuinely cannot be evaluated after any single row insert — only once
-- every row for that transaction_id exists does "sums to zero" become a
-- meaningful question. A CONSTRAINT TRIGGER is the only way to express
-- "check this once, at commit, across rows inserted earlier in the same
-- transaction."
CREATE OR REPLACE FUNCTION ledger_check_transaction_balance() RETURNS TRIGGER AS $$
DECLARE
    total BIGINT;
BEGIN
    SELECT SUM(amount) INTO total FROM entries WHERE transaction_id = NEW.transaction_id;
    IF total <> 0 THEN
        RAISE EXCEPTION USING
            ERRCODE = 'LG002',
            MESSAGE = format('ledger: transaction %s does not balance to zero (sum = %s)', NEW.transaction_id, total);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER entries_balance_check
    AFTER INSERT ON entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION ledger_check_transaction_balance();

-- Invariant 3: a transaction cannot span more than one currency.
--
-- Also deferred for the same reason as invariant 2 — the full set of
-- accounts a transaction touches is only known once all its entries exist.
-- This module has no FX conversion logic, so a multi-currency transaction
-- isn't a feature gap being enforced around, it's simply not a meaningful
-- operation here.
CREATE OR REPLACE FUNCTION ledger_check_transaction_currency() RETURNS TRIGGER AS $$
DECLARE
    currency_count INT;
BEGIN
    SELECT COUNT(DISTINCT a.currency) INTO currency_count
    FROM entries e
    JOIN accounts a ON a.id = e.account_id
    WHERE e.transaction_id = NEW.transaction_id;

    IF currency_count > 1 THEN
        RAISE EXCEPTION USING
            ERRCODE = 'LG003',
            MESSAGE = format('ledger: transaction %s spans more than one currency', NEW.transaction_id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER entries_currency_check
    AFTER INSERT ON entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION ledger_check_transaction_currency();

-- Invariant 4: no overdraft unless the account explicitly allows it.
--
-- The Go client pre-checks this before ever issuing the UPDATE, so the
-- common case never reaches this trigger — but the pre-check is the fast
-- path, not the guarantee. This is the backstop that makes "no negative
-- balance without allow_overdraft" actually unbreakable: it holds even if
-- the pre-check has a bug, even against a concurrent writer that
-- isn't this module's own locking-aware code, even against a direct SQL
-- UPDATE.
CREATE OR REPLACE FUNCTION ledger_check_no_overdraft() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.balance < 0 AND NOT NEW.allow_overdraft THEN
        RAISE EXCEPTION USING
            ERRCODE = 'LG004',
            MESSAGE = format('ledger: account %s would go negative (balance %s) and does not allow overdraft', NEW.external_ref, NEW.balance);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER accounts_no_overdraft
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION ledger_check_no_overdraft();
