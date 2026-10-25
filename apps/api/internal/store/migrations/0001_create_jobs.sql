-- Jobs keep a snapshot of the printer and every option, so a job's history
-- stays accurate even if the printer configuration changes later.
CREATE TABLE jobs (
    id                  TEXT    PRIMARY KEY,
    printer_id          TEXT    NOT NULL,
    printer_name        TEXT    NOT NULL,
    printer_model       TEXT    NOT NULL,
    printer_simulated   INTEGER NOT NULL CHECK (printer_simulated IN (0, 1)),
    document_name       TEXT    NOT NULL,
    document_size_bytes INTEGER NOT NULL,
    copies              INTEGER NOT NULL,
    color_mode          TEXT    NOT NULL,
    paper_size          TEXT    NOT NULL,
    orientation         TEXT    NOT NULL,
    duplex              INTEGER NOT NULL CHECK (duplex IN (0, 1)),
    page_ranges         TEXT    NOT NULL,
    status              TEXT    NOT NULL,
    provider_job_id     TEXT    NOT NULL,
    failure_reason      TEXT    NOT NULL,
    created_at          TEXT    NOT NULL, -- RFC 3339, UTC
    updated_at          TEXT    NOT NULL  -- RFC 3339, UTC
) STRICT;

-- Startup recovery looks jobs up by status.
CREATE INDEX jobs_status ON jobs (status);

-- One row per status a job entered, in order (seq starts at 0).
CREATE TABLE job_transitions (
    job_id TEXT    NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    seq    INTEGER NOT NULL,
    status TEXT    NOT NULL,
    at     TEXT    NOT NULL, -- RFC 3339, UTC
    PRIMARY KEY (job_id, seq)
) STRICT;
