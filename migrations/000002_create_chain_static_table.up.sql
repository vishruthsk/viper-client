CREATE TABLE IF NOT EXISTS chain_static (
  id SERIAL PRIMARY KEY,
  chain_id INTEGER NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  symbol VARCHAR(50) NOT NULL,
  network_type VARCHAR(20) NOT NULL, -- mainnet, testnet
  is_evm BOOLEAN NOT NULL DEFAULT FALSE,
  chain_details JSONB,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
); 