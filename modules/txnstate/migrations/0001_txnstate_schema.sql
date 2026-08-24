-- Numbered "0001" relative to this module's own migration history, not
-- a project-wide sequence -- see modules/base's generated README for
-- the numbering-gotcha note that applies whenever more than one
-- schema-owning module (ledger, audit, txnstate) is selected together.

CREATE TABLE transaction_transitions (
    id              BIGSERIAL PRIMARY KEY,
    entity_id       TEXT NOT NULL,
    -- '' (empty string) means "did not exist before this transition" --
    -- the state of an entity_id that has never transitioned. Reserved
    -- by convention in the Go client (Tracker), not enforced here as a
    -- distinct schema-level rule, since any TEXT NOT NULL already
    -- permits an empty string.
    from_state      TEXT NOT NULL,
    to_state        TEXT NOT NULL,
    -- NULL (not the JSON literal "null") when the caller passed a Go
    -- nil for Metadata -- same distinction modules/audit's schema
    -- makes for Before/After, reused here rather than reinvented.
    metadata        JSONB,
    transitioned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every current-state and history lookup is "give me this entity_id's
-- rows, in the order they happened" -- this index is what makes both
-- of those cheap without a separate cached "current state" table the
-- way modules/ledger caches accounts.balance. Summing many rows (what
-- ledger's cache avoids) is a different cost than finding the latest
-- of them (what this index already makes fast), so the extra table
-- ledger needs doesn't have an analog here.
CREATE INDEX transaction_transitions_entity_id_idx ON transaction_transitions (entity_id, id);

-- Invariant: append-only, identical mechanism and intent to ledger's
-- and audit's own append-only triggers -- a transition history that
-- could be edited after the fact isn't a history. SQLSTATE "TS001" is
-- this module's own prefix, confirmed against every other custom code
-- in this catalog before picking it: ledger uses LG001-LG004, audit
-- uses AU001, neither collides with TS.
CREATE OR REPLACE FUNCTION txnstate_forbid_update_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION USING
        ERRCODE = 'TS001',
        MESSAGE = format('txnstate: %s is append-only; %s is not allowed', TG_TABLE_NAME, TG_OP);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER transaction_transitions_append_only
    BEFORE UPDATE OR DELETE ON transaction_transitions
    FOR EACH ROW
    EXECUTE FUNCTION txnstate_forbid_update_delete();
