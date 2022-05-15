INSERT INTO users (id, name) VALUES (1, 'Josh');
INSERT INTO users (id, name) VALUES (2, 'Ruth');
INSERT INTO users (id, name) VALUES (3, 'Michael');
INSERT INTO users (id, name) VALUES (4, 'Lila');

INSERT INTO email (id, subject, user_id, status) VALUES (1, 'Welcome to Concurrency Lab!', 1, 'UNREAD');

INSERT INTO user_email_stats (unread, user_id) VALUES (1, 1);
INSERT INTO user_email_stats (unread, user_id) VALUES (0, 2);
INSERT INTO user_email_stats (unread, user_id) VALUES (0, 3);
INSERT INTO user_email_stats (unread, user_id) VALUES (0, 4);
