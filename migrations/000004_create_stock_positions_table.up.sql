CREATE TABLE stock_positions (
  id SERIAL PRIMARY KEY,
  stock_code VARCHAR(50) NOT NULL,
  buy_price FLOAT NOT NULL,
  take_profit_price FLOAT,     
  stop_loss_price FLOAT,        
  buy_date DATE NOT NULL,
  max_holding_period_days INT,
  is_alert_triggered BOOLEAN DEFAULT false,
  last_alerted_at TIMESTAMP,
  alert_count INT DEFAULT 0,
  created_at TIMESTAMP DEFAULT now()
);