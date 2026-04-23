-- migrate:up
alter table users add column email varchar(255);

-- migrate:down
