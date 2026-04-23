-- migrate:up
create table comments (
  id integer primary key,
  post_id integer,
  author varchar(255),
  content text,
  created_at datetime default current_timestamp
);

-- migrate:down
drop table comments;
