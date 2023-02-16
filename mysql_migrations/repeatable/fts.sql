DROP TRIGGER IF EXISTS note_after_update_trigger;

DROP TRIGGER IF EXISTS note_after_delete_trigger;

DROP TRIGGER IF EXISTS note_after_insert_trigger;

DROP TABLE IF EXISTS note_fts;

CREATE TABLE IF NOT EXISTS note_fts (
    user_id BINARY(16) NOT NULL
    ,note_number INT NOT NULL
    ,body VARCHAR(65536)

    ,PRIMARY KEY (user_id, note_number)
    ,FULLTEXT INDEX note_fts_body_idx (body)
);

CREATE TRIGGER note_after_insert_trigger AFTER INSERT ON note FOR EACH ROW BEGIN
    INSERT INTO note_fts (user_id, note_number, body) VALUES (NEW.user_id, NEW.note_number, NEW.body);
END;

CREATE TRIGGER note_after_update_trigger AFTER UPDATE ON note FOR EACH ROW BEGIN
    IF OLD.body <> NEW.body THEN
        UPDATE note_fts
        SET body = NEW.body
        WHERE user_id = NEW.user_id AND note_number = NEW.note_number;
    END IF;
END;

CREATE TRIGGER note_after_delete_trigger AFTER DELETE ON note FOR EACH ROW BEGIN
    DELETE FROM note_fts WHERE user_id = OLD.user_id AND note_number = OLD.note_number;
END;
