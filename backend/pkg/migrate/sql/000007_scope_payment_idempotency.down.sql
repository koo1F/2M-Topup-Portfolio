-- Resolve cross-user key collisions before restoring global uniqueness.
ALTER TABLE payments ADD CONSTRAINT payments_idempotency_unique UNIQUE (idempotency_key);
ALTER TABLE payments DROP CONSTRAINT payments_user_idempotency_unique;
ALTER TABLE payments DROP COLUMN method;
