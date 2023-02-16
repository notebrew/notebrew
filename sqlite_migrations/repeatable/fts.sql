DROP TRIGGER IF EXISTS note_after_update_trigger;

DROP TRIGGER IF EXISTS note_after_delete_trigger;

DROP TRIGGER IF EXISTS note_after_insert_trigger;

DROP TABLE IF EXISTS note_fts;

CREATE VIRTUAL TABLE IF NOT EXISTS note_fts USING FTS5 (
    body
    ,content='note'
    ,content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS note_after_insert_trigger AFTER INSERT ON note BEGIN
    INSERT INTO note_fts (ROWID, body) VALUES (NEW.ROWID, NEW.body);
END;

CREATE TRIGGER IF NOT EXISTS note_after_delete_trigger AFTER DELETE ON note BEGIN
    INSERT INTO note_fts (note_fts, ROWID, body) VALUES ('delete', OLD.ROWID, OLD.body);
END;

CREATE TRIGGER IF NOT EXISTS note_after_update_trigger AFTER UPDATE ON note BEGIN
    INSERT INTO note_fts (note_fts, ROWID, body) VALUES ('delete', OLD.ROWID, OLD.body);
    INSERT INTO note_fts (ROWID, body) VALUES (NEW.ROWID, NEW.body);
END;
