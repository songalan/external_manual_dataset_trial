-- migrate:up
create table audit_log (
  id integer primary key,
  table_name varchar(100),
  action varchar(20),
  performed_at datetime default current_timestamp
);

-- migrate:down
drop table audit_log;
