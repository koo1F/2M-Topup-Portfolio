ALTER TABLE payments ADD COLUMN method VARCHAR(16) NOT NULL DEFAULT 'promptpay';
UPDATE payments SET method = 'card' WHERE left(gateway_reference, 3) = 'cs_';
ALTER TABLE payments ADD CONSTRAINT payments_method_check CHECK (method IN ('card', 'promptpay'));
ALTER TABLE payments DROP CONSTRAINT payments_idempotency_unique;
ALTER TABLE payments ADD CONSTRAINT payments_user_idempotency_unique UNIQUE (user_id, idempotency_key);
