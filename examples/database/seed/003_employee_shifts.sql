DELETE FROM shift;
DELETE FROM employee;
DELETE FROM employee_shift;

INSERT INTO shift (id) VALUES (1234);
INSERT INTO employee (id, name) VALUES (1, 'Alice');
INSERT INTO employee (id, name) VALUES (2, 'Bob');
INSERT INTO employee_shift (shift_id, employee_id) VALUES (1234, 1);
INSERT INTO employee_shift (shift_id, employee_id) VALUES (1234, 2);
