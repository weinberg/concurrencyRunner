CREATE TABLE IF NOT EXISTS users
(
    id   serial
        CONSTRAINT users_pk PRIMARY KEY,
    name text
);

CREATE UNIQUE INDEX IF NOT EXISTS users_id_uindex ON users (id);

CREATE TABLE IF NOT EXISTS user_email_stats
(
    unread  integer,
    user_id integer
        CONSTRAINT user_email_stats___fk_users_id REFERENCES users ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS email
(
    id      serial
        CONSTRAINT email_pk PRIMARY KEY,
    subject varchar,
    user_id integer
        CONSTRAINT email___fk_user_id REFERENCES users ON DELETE CASCADE,
    status  varchar(20)
);

CREATE UNIQUE INDEX IF NOT EXISTS email_id_uindex ON email (id);

CREATE TABLE IF NOT EXISTS accounts
(
    id      serial
        CONSTRAINT accounts_pk PRIMARY KEY,
    name    text,
    balance float
);

CREATE TABLE IF NOT EXISTS shift
(
    id      serial
        CONSTRAINT shift_pk PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS employee
(
    id      serial
        CONSTRAINT employee_pk PRIMARY KEY,
    name    text
);

CREATE TABLE IF NOT EXISTS employee_shift
(
    shift_id integer
        CONSTRAINT employee_shift___fk_shift_id REFERENCES shift ON DELETE CASCADE,
    employee_id integer
        CONSTRAINT employee_shift__fk_employee_id REFERENCES employee ON DELETE CASCADE
);

