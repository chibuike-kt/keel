-- Numbered "0001" relative to this module's own migration history, not
-- a project-wide sequence -- a project selecting both this module and
-- ledger will have two files both named "0001_*_schema.sql". Most
-- tools sort by full filename and handle that fine; a tool that treats
-- the leading digits as a single global, strictly-unique version
-- number (e.g. golang-migrate's default numeric scheme) will not.
-- Check your migration tool's assumptions before running both.
--
-- Append-only audit trail. Unlike modules/ledger's schema, there is no
-- legitimate mutation path for a row here at all -- ledger's accounts
-- table still needs an UPDATE trigger to police overdrafts, but an
-- audit record has no analogous "state that legitimately changes";
-- once written, it never needs to change, so the append-only trigger
-- below has no carve-out to make.

CREATE TABLE audit_events (
    id          BIGSERIAL PRIMARY KEY,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    -- NULL (not the JSON literal "null") when the caller passed a Go
    -- nil for Before/After -- e.g. a "record created" event has no
    -- prior state. Keeping real SQL NULL distinct from a stored JSON
    -- null is what lets a later reader tell "this event has no Before"
    -- apart from "this event's Before really was JSON null".
    before      JSONB,
    after       JSONB,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes anticipate how a caller will query this table later (by
-- actor, by action) even though this module exposes no read API of
-- its own -- querying audit_events is entirely the caller's own SQL,
-- per this module's scope, but an unindexed table that grows
-- unboundedly would be a real operational trap for whoever eventually
-- writes that SQL.
CREATE INDEX audit_events_actor_idx ON audit_events (actor);
CREATE INDEX audit_events_action_idx ON audit_events (action);

-- Invariant: append-only. Audit records exist specifically so they
-- outlive and don't depend on whatever retention an application log
-- stream happens to have -- an UPDATE or DELETE here would silently
-- defeat that whole premise. This can't be a Go-level rule: it has to
-- hold even against a bug in a future version of this client, a
-- one-off psql session, or a different service later given write
-- access to this database.
CREATE OR REPLACE FUNCTION audit_forbid_update_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION USING
        ERRCODE = 'AU001',
        MESSAGE = format('audit: %s is append-only; %s is not allowed', TG_TABLE_NAME, TG_OP);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_forbid_update_delete();
