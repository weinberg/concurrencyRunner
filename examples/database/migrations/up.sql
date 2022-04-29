

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


