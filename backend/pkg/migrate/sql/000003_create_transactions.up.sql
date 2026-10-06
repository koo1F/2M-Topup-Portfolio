CREATE TYPE transaction_type   AS ENUM ('DEPOSIT', 'WITHDRAW', 'PAYMENT', 'REFUND');
CREATE TYPE transaction_status AS ENUM ('PENDING', 'SUCCESS', 'FAILED');

CREATE TABLE transactions (
  id           BIGSERIAL          PRIMARY KEY,
  wallet_id    BIGINT             NOT NULL,
  type         transaction_type   NOT NULL,
  amount       NUMERIC(15, 2)     NOT NULL,
  status       transaction_status NOT NULL DEFAULT 'PENDING',
  reference_id VARCHAR(100)       NOT NULL,
  metadata     JSONB,
  created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),

  CONSTRAINT transactions_wallet_fk        FOREIGN KEY (wallet_id) REFERENCES wallets(id),
  CONSTRAINT transactions_amount_positive  CHECK (amount > 0),
  CONSTRAINT transactions_reference_unique UNIQUE (reference_id)
);

CREATE INDEX idx_transactions_wallet_id    ON transactions(wallet_id);
CREATE INDEX idx_transactions_created_at  ON transactions(created_at DESC);
CREATE INDEX idx_transactions_reference_id ON transactions(reference_id);
