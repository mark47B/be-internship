-- Триггеры
DROP TRIGGER IF EXISTS trg_protect_pr_status ON pull_requests;
DROP FUNCTION IF EXISTS fn_protect_pr_status();

DROP TRIGGER IF EXISTS trg_review_assignment_checks ON review_assignments;
DROP FUNCTION IF EXISTS fn_review_assignment_checks();

-- Индексы
DROP INDEX IF EXISTS idx_pull_requests_open;
DROP INDEX IF EXISTS idx_pull_requests_status;
DROP INDEX IF EXISTS idx_pull_requests_author;
DROP INDEX IF EXISTS idx_review_assignments_reviewer;
DROP INDEX IF EXISTS idx_users_team_name;

-- Таблицы
DROP TABLE IF EXISTS review_assignments;
DROP TABLE IF EXISTS pull_requests;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS teams;
