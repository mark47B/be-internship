-- TEAMS
CREATE TABLE IF NOT EXISTS teams (
    name TEXT PRIMARY KEY
);

-- USERS
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    team_name TEXT REFERENCES teams(name) ON DELETE SET NULL
);

-- PULL REQUESTS
CREATE TABLE IF NOT EXISTS pull_requests (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    author_id TEXT REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('OPEN', 'MERGED')),
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    merged_at TIMESTAMP
);

-- REVIEW ASSIGNMENTS
CREATE TABLE IF NOT EXISTS review_assignments (
    pr_id TEXT NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    reviewer_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (pr_id, reviewer_id)
);

-- -------------------------
-- Индексы для ускорения запросов
-- -------------------------

-- Поиск пользователей по команде
CREATE INDEX IF NOT EXISTS idx_users_team_name ON users(team_name);

-- Быстрый поиск по reviewer -> список PR
CREATE INDEX IF NOT EXISTS idx_review_assignments_reviewer ON review_assignments(reviewer_id);

-- По автору PR
CREATE INDEX IF NOT EXISTS idx_pull_requests_author ON pull_requests(author_id);

-- По статусу (частые агрегаты open/merged)
CREATE INDEX IF NOT EXISTS idx_pull_requests_status ON pull_requests(status);

-- -------------------------
-- Функции/триггеры для целостности
-- -------------------------

/*
1) Проверки бизнес-правил на уровне БД для review_assignments:
   - нельзя назначить автора PR ревьювером
   - нельзя добавлять/удалять/изменять назначений если PR.status = 'MERGED'
   - нельзя иметь > 2 ревьюверов на PR (BEFORE INSERT)
*/

CREATE OR REPLACE FUNCTION fn_review_assignment_checks() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  pr_author TEXT;
  pr_status TEXT;
  cnt INT;
BEGIN
  -- Получаем автора и статус PR
  SELECT author_id, status INTO pr_author, pr_status FROM pull_requests WHERE id = COALESCE(NEW.pr_id, OLD.pr_id) FOR SHARE;

  IF pr_status = 'MERGED' THEN
    RAISE EXCEPTION 'cannot change review assignments for MERGED PR %', COALESCE(NEW.pr_id, OLD.pr_id);
  END IF;

  -- Проверка: reviewer != author
  IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
    IF NEW.reviewer_id = pr_author THEN
      RAISE EXCEPTION 'cannot assign PR author % as reviewer for PR %', pr_author, NEW.pr_id;
    END IF;
  END IF;

  -- Проверка лимита на количество ревьюверов (для INSERT)
  IF TG_OP = 'INSERT' THEN
    SELECT COUNT(*) INTO cnt FROM review_assignments WHERE pr_id = NEW.pr_id;
    IF cnt >= 2 THEN
      RAISE EXCEPTION 'cannot assign more than 2 reviewers to PR %', NEW.pr_id;
    END IF;
  END IF;

  -- Для DELETE возвращаем OLD, для INSERT/UPDATE - NEW
  IF TG_OP = 'DELETE' THEN
    RETURN OLD;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_review_assignment_checks
BEFORE INSERT OR UPDATE OR DELETE ON review_assignments
FOR EACH ROW
EXECUTE FUNCTION fn_review_assignment_checks();

-- -------------------------
/*
2) Защита: нельзя изменить status PR с MERGED обратно
*/
CREATE OR REPLACE FUNCTION fn_protect_pr_status() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'UPDATE' THEN
    IF OLD.status = 'MERGED' AND NEW.status <> 'MERGED' THEN
      RAISE EXCEPTION 'cannot change status from MERGED to % for PR %', NEW.status, OLD.id;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_protect_pr_status
BEFORE UPDATE ON pull_requests
FOR EACH ROW
EXECUTE FUNCTION fn_protect_pr_status();
