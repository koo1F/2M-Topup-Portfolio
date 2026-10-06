CREATE TABLE wallets (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT      NOT NULL,
  balance    NUMERIC(15, 2) NOT NULL DEFAULT 0.00,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT wallets_user_fk      FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT wallets_user_unique  UNIQUE (user_id),
  CONSTRAINT wallets_balance_positive CHECK (balance >= 0)
);
