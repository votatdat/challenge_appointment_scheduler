BEGIN;

DO $$
BEGIN
    IF (SELECT count(*) FROM dealerships WHERE id IN (1, 2)) <> 2 THEN
        RAISE EXCEPTION 'expected two seeded dealerships';
    END IF;

    IF (SELECT count(*) FROM technicians WHERE dealership_id = 1) <> 3 THEN
        RAISE EXCEPTION 'expected three technicians at dealership 1';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM technician_qualifications
        WHERE technician_id = 3
    ) THEN
        RAISE EXCEPTION 'technician 3 must remain unqualified';
    END IF;

    IF (
        SELECT count(*)
        FROM technicians AS t
        JOIN technician_qualifications AS tq ON tq.technician_id = t.id
        WHERE t.dealership_id = 1
          AND tq.service_type_id = 1
    ) < 2 THEN
        RAISE EXCEPTION 'expected alternative qualified technicians at dealership 1';
    END IF;

    IF (SELECT count(*) FROM service_bays WHERE dealership_id = 1) < 2 THEN
        RAISE EXCEPTION 'expected alternative bays at dealership 1';
    END IF;

    IF (
        SELECT count(*)
        FROM pg_indexes
        WHERE schemaname = current_schema()
          AND indexname IN (
              'idx_vehicles_customer',
              'idx_technicians_dealership',
              'idx_technician_qualifications_service',
              'idx_service_bays_dealership',
              'idx_appointments_technician_interval',
              'idx_appointments_bay_interval'
          )
    ) <> 6 THEN
        RAISE EXCEPTION 'expected all schema lookup indexes';
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM appointments
        WHERE id = 1
          AND technician_id = 1
          AND service_bay_id = 1
          AND start_time = '2099-01-15 10:00:00+00'
          AND end_time = '2099-01-15 11:00:00+00'
    ) THEN
        RAISE EXCEPTION 'expected the seeded occupied resource pair';
    END IF;
END;
$$;

DO $$
BEGIN
    BEGIN
        INSERT INTO service_types (id, name, duration_minutes)
        VALUES (9001, 'Invalid Duration', 0);
        RAISE EXCEPTION 'expected a positive-duration check violation';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO appointments (
            id, customer_id, vehicle_id, dealership_id, service_type_id,
            technician_id, service_bay_id, start_time, end_time
        ) VALUES (
            9001, 2, 1, 1, 1, 1, 2,
            '2099-02-01 10:00:00+00', '2099-02-01 11:00:00+00'
        );
        RAISE EXCEPTION 'expected a customer/vehicle foreign-key violation';
    EXCEPTION
        WHEN foreign_key_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO appointments (
            id, customer_id, vehicle_id, dealership_id, service_type_id,
            technician_id, service_bay_id, start_time, end_time
        ) VALUES (
            9002, 1, 1, 1, 1, 3, 2,
            '2099-02-01 10:00:00+00', '2099-02-01 11:00:00+00'
        );
        RAISE EXCEPTION 'expected an unqualified-technician foreign-key violation';
    EXCEPTION
        WHEN foreign_key_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO appointments (
            id, customer_id, vehicle_id, dealership_id, service_type_id,
            technician_id, service_bay_id, start_time, end_time
        ) VALUES (
            9003, 1, 1, 1, 1, 4, 2,
            '2099-02-01 10:00:00+00', '2099-02-01 11:00:00+00'
        );
        RAISE EXCEPTION 'expected a technician/dealership foreign-key violation';
    EXCEPTION
        WHEN foreign_key_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO appointments (
            id, customer_id, vehicle_id, dealership_id, service_type_id,
            technician_id, service_bay_id, start_time, end_time
        ) VALUES (
            9004, 1, 1, 1, 1, 1, 2,
            '2099-02-01 11:00:00+00', '2099-02-01 10:00:00+00'
        );
        RAISE EXCEPTION 'expected an appointment-interval check violation';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;
END;
$$;

SELECT 'dealerships' AS entity, count(*) AS row_count FROM dealerships
UNION ALL
SELECT 'customers', count(*) FROM customers
UNION ALL
SELECT 'vehicles', count(*) FROM vehicles
UNION ALL
SELECT 'service_types', count(*) FROM service_types
UNION ALL
SELECT 'technicians', count(*) FROM technicians
UNION ALL
SELECT 'qualifications', count(*) FROM technician_qualifications
UNION ALL
SELECT 'service_bays', count(*) FROM service_bays
UNION ALL
SELECT 'appointments', count(*) FROM appointments
ORDER BY entity;

ROLLBACK;
