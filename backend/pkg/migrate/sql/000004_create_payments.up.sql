CREATE TYPE payment_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED', 'EXPIRED');

CREATE TABLE payments (
  id                BIGSERIAL      PRIMARY KEY,
  user_id           BIGINT         NOT NULL,
  amount            NUMERIC(15, 2) NOT NULL,
  status            payment_status NOT NULL DEFAULT 'PENDING',
  gateway_reference VARCHAR(100),
  idempotency_key   VARCHAR(100)   NOT NULL,
  expires_at        TIMESTAMPTZ    NOT NULL,
  created_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
  paid_at           TIMESTAMPTZ,

  CONSTRAINT payments_user_fk             FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT payments_amount_positive     CHECK (amount > 0),
  CONSTRAINT payments_idempotency_unique  UNIQUE (idempotency_key)
);

CREATE INDEX idx_payments_user_id    ON payments(user_id);
CREATE INDEX idx_payments_status     ON payments(status);
-- Partial index: only index PENDING payments for expiry checks
CREATE INDEX idx_payments_expires_at ON payments(expires_at) WHERE status = 'PENDING';
