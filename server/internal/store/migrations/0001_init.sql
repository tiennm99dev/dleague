-- 0001_init.sql
-- Initial schema for dleague.
-- Owner: Phase D of plans/260505-1319-mysql-heatwave-integration/.
-- Idempotent: re-applying must produce no errors and no changes.

CREATE TABLE IF NOT EXISTS users (
  id              BINARY(16) NOT NULL,
  email           VARCHAR(254) NOT NULL,
  password_hash   VARCHAR(120) NOT NULL,
  display_name    VARCHAR(64) NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_email_lower ((LOWER(email)))
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS sessions (
  token       BINARY(32) NOT NULL,
  user_id     BINARY(16) NOT NULL,
  expires_at  TIMESTAMP NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (token),
  KEY idx_sessions_user (user_id),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id)
    REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS puzzles (
  puzzle_date   DATE NOT NULL,
  game_id       VARCHAR(32) NOT NULL,
  seed          BIGINT NOT NULL,
  answer_hash   BINARY(32) NOT NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (puzzle_date, game_id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS matches (
  id            BINARY(16) NOT NULL,
  kind          ENUM('async','sync','daily') NOT NULL,
  game_id       VARCHAR(32) NOT NULL,
  puzzle_date   DATE NOT NULL,
  creator_id    BINARY(16) NOT NULL,
  joiner_id     BINARY(16) NULL,
  status        ENUM('open','in_progress','completed','expired') NOT NULL DEFAULT 'open',
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  completed_at  TIMESTAMP NULL,
  PRIMARY KEY (id),
  KEY idx_matches_open (status, kind, created_at),
  KEY idx_matches_creator (creator_id),
  KEY idx_matches_joiner (joiner_id),
  CONSTRAINT fk_matches_creator FOREIGN KEY (creator_id) REFERENCES users(id),
  CONSTRAINT fk_matches_joiner  FOREIGN KEY (joiner_id)  REFERENCES users(id),
  CONSTRAINT fk_matches_puzzle  FOREIGN KEY (puzzle_date, game_id)
    REFERENCES puzzles(puzzle_date, game_id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS attempts (
  id            BINARY(16) NOT NULL,
  match_id      BINARY(16) NOT NULL,
  user_id       BINARY(16) NOT NULL,
  attempts_used SMALLINT UNSIGNED NOT NULL,
  duration_ms   INT UNSIGNED NOT NULL,
  won           TINYINT(1) NOT NULL,
  state         JSON NOT NULL,
  finished_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_attempt_per_user_match (match_id, user_id),
  KEY idx_attempts_user (user_id),
  KEY idx_attempts_won_user (user_id, won),
  CONSTRAINT fk_attempts_match FOREIGN KEY (match_id) REFERENCES matches(id) ON DELETE CASCADE,
  CONSTRAINT fk_attempts_user  FOREIGN KEY (user_id)  REFERENCES users(id)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
