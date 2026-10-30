-- FL-OTP01: additive email authentication; Google identities and sessions remain intact.
CREATE TABLE email_otp_challenges (
  id TEXT PRIMARY KEY,
  email_key TEXT NOT NULL,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  email_binding TEXT NOT NULL,
  verifier BLOB NOT NULL CHECK (length(verifier) = 32),
  delivery TEXT NOT NULL CHECK (delivery IN ('pending','delivered','failed')),
  attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  consumed INTEGER NOT NULL DEFAULT 0 CHECK (consumed IN (0,1)),
  superseded INTEGER NOT NULL DEFAULT 0 CHECK (superseded IN (0,1)),
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_email_otp_expiry ON email_otp_challenges(expires_at);
CREATE INDEX idx_email_otp_binding ON email_otp_challenges(email_key, superseded);

CREATE TABLE email_verified_identities (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  email_binding TEXT NOT NULL,
  verified_at TEXT NOT NULL,
  PRIMARY KEY (user_id, email_binding)
);

-- Keys are purpose-separated HMACs, never plaintext email addresses or IPs.
CREATE TABLE email_otp_rate_buckets (
  bucket_key TEXT PRIMARY KEY,
  count INTEGER NOT NULL CHECK (count > 0),
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_email_otp_rate_expiry ON email_otp_rate_buckets(expires_at);
