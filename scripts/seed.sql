-- Seed data for local development and demo purposes.

-- Households
INSERT INTO households (id, owner_name, address)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'Budi Santoso',    'Jl. Merdeka No. 1, Jakarta'),
  ('22222222-2222-2222-2222-222222222222', 'Siti Rahayu',     'Jl. Pahlawan No. 5, Bandung'),
  ('33333333-3333-3333-3333-333333333333', 'Ahmad Fauzi',     'Jl. Sudirman No. 10, Medan'),
  ('44444444-4444-4444-4444-444444444444', 'Dewi Lestari',    'Jl. Diponegoro No. 3, Yogyakarta'),
  ('55555555-5555-5555-5555-555555555555', 'Rudi Hartono',    'Jl. Imam Bonjol No. 7, Semarang')
ON CONFLICT (id) DO NOTHING;

-- Waste Pickups (various types and statuses)
INSERT INTO waste_pickups (id, household_id, type, status, pickup_date, safety_check)
VALUES
  -- Budi: organic completed, plastic pending
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 'organic',    'completed', NOW() - INTERVAL '5 days', false),
  ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', '11111111-1111-1111-1111-111111111111', 'plastic',    'pending',   NULL,                       false),
  -- Siti: electronic completed with safety check, paper scheduled
  ('cccccccc-cccc-cccc-cccc-cccccccccccc', '22222222-2222-2222-2222-222222222222', 'electronic', 'completed', NOW() - INTERVAL '10 days', true),
  ('dddddddd-dddd-dddd-dddd-dddddddddddd', '22222222-2222-2222-2222-222222222222', 'paper',      'scheduled', NOW() + INTERVAL '2 days',  false),
  -- Ahmad: canceled pickup
  ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', '33333333-3333-3333-3333-333333333333', 'organic',    'canceled',  NULL,                       false),
  -- Dewi: pending pickup
  ('ffffffff-ffff-ffff-ffff-ffffffffffff', '44444444-4444-4444-4444-444444444444', 'paper',      'pending',   NULL,                       false),
  -- Rudi: completed organic and electronic
  ('a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', '55555555-5555-5555-5555-555555555555', 'organic',  'completed', NOW() - INTERVAL '7 days',  false),
  ('b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2', '55555555-5555-5555-5555-555555555555', 'electronic','completed', NOW() - INTERVAL '3 days',  true)
ON CONFLICT (id) DO NOTHING;

-- Payments (for completed pickups)
INSERT INTO payments (id, household_id, waste_id, amount, status, payment_date, proof_file_url)
VALUES
  -- Budi's organic pickup → paid
  ('11111111-aaaa-aaaa-aaaa-111111111111', '11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 50000.00,  'paid',    NOW() - INTERVAL '4 days', 'https://storage.example.com/proofs/budi-organic.jpg'),
  -- Siti's electronic pickup → pending (not yet confirmed)
  ('22222222-cccc-cccc-cccc-222222222222', '22222222-2222-2222-2222-222222222222', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 100000.00, 'pending', NULL,                        NULL),
  -- Rudi's organic → paid
  ('55555555-a1a1-a1a1-a1a1-555555555555', '55555555-5555-5555-5555-555555555555', 'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 50000.00, 'paid',    NOW() - INTERVAL '6 days', 'https://storage.example.com/proofs/rudi-organic.jpg'),
  -- Rudi's electronic → pending
  ('55555555-b2b2-b2b2-b2b2-555555555555', '55555555-5555-5555-5555-555555555555', 'b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2', 100000.00,'pending', NULL,                        NULL)
ON CONFLICT (id) DO NOTHING;
