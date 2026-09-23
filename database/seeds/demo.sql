BEGIN;

INSERT INTO dealerships (id, name) VALUES
    (1, 'Central Service Centre'),
    (2, 'Riverside Service Centre')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name;

INSERT INTO customers (id, full_name) VALUES
    (1, 'Alice Nguyen'),
    (2, 'Ben Carter')
ON CONFLICT (id) DO UPDATE SET
    full_name = EXCLUDED.full_name;

INSERT INTO vehicles (id, customer_id, registration_number, make, model) VALUES
    (1, 1, 'KL-001', 'Toyota', 'Corolla'),
    (2, 2, 'KL-002', 'Ford', 'Focus'),
    (3, 1, 'KL-003', 'Honda', 'Civic')
ON CONFLICT (id) DO UPDATE SET
    customer_id = EXCLUDED.customer_id,
    registration_number = EXCLUDED.registration_number,
    make = EXCLUDED.make,
    model = EXCLUDED.model;

INSERT INTO service_types (id, name, duration_minutes) VALUES
    (1, 'Oil Change', 60),
    (2, 'Brake Inspection', 90),
    (3, 'Wheel Alignment', 45)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    duration_minutes = EXCLUDED.duration_minutes;

INSERT INTO technicians (id, dealership_id, full_name) VALUES
    (1, 1, 'Alex Morgan'),
    (2, 1, 'Priya Shah'),
    (3, 1, 'Sam Lee'),
    (4, 2, 'Jordan Kim'),
    (5, 2, 'Taylor Brown')
ON CONFLICT (id) DO UPDATE SET
    dealership_id = EXCLUDED.dealership_id,
    full_name = EXCLUDED.full_name;

INSERT INTO technician_qualifications (technician_id, service_type_id) VALUES
    (1, 1),
    (1, 2),
    (2, 1),
    (2, 3),
    (4, 1),
    (4, 2),
    (5, 3)
ON CONFLICT DO NOTHING;

INSERT INTO service_bays (id, dealership_id, name) VALUES
    (1, 1, 'Bay 1'),
    (2, 1, 'Bay 2'),
    (3, 2, 'Bay 1'),
    (4, 2, 'Bay 2')
ON CONFLICT (id) DO UPDATE SET
    dealership_id = EXCLUDED.dealership_id,
    name = EXCLUDED.name;

INSERT INTO appointments (
    id,
    customer_id,
    vehicle_id,
    dealership_id,
    service_type_id,
    technician_id,
    service_bay_id,
    start_time,
    end_time,
    status,
    created_at
) VALUES (
    1,
    1,
    1,
    1,
    1,
    1,
    1,
    '2099-01-15 10:00:00+00',
    '2099-01-15 11:00:00+00',
    'CONFIRMED',
    '2026-09-23 00:00:00+00'
)
ON CONFLICT (id) DO UPDATE SET
    customer_id = EXCLUDED.customer_id,
    vehicle_id = EXCLUDED.vehicle_id,
    dealership_id = EXCLUDED.dealership_id,
    service_type_id = EXCLUDED.service_type_id,
    technician_id = EXCLUDED.technician_id,
    service_bay_id = EXCLUDED.service_bay_id,
    start_time = EXCLUDED.start_time,
    end_time = EXCLUDED.end_time,
    status = EXCLUDED.status,
    created_at = EXCLUDED.created_at;

SELECT setval(pg_get_serial_sequence('dealerships', 'id'), (SELECT max(id) FROM dealerships), true);
SELECT setval(pg_get_serial_sequence('customers', 'id'), (SELECT max(id) FROM customers), true);
SELECT setval(pg_get_serial_sequence('vehicles', 'id'), (SELECT max(id) FROM vehicles), true);
SELECT setval(pg_get_serial_sequence('service_types', 'id'), (SELECT max(id) FROM service_types), true);
SELECT setval(pg_get_serial_sequence('technicians', 'id'), (SELECT max(id) FROM technicians), true);
SELECT setval(pg_get_serial_sequence('service_bays', 'id'), (SELECT max(id) FROM service_bays), true);
SELECT setval(pg_get_serial_sequence('appointments', 'id'), (SELECT max(id) FROM appointments), true);

COMMIT;
