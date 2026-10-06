DROP INDEX IF EXISTS idx_payments_expires_at;
DROP INDEX IF EXISTS idx_payments_status;
DROP INDEX IF EXISTS idx_payments_user_id;
DROP TABLE IF EXISTS payments;
DROP TYPE IF EXISTS payment_status;
