-- migrate:up
create table tags (
  id integer primary key,
  name varchar(100) unique not null
);
create table post_tags (
  post_id integer,
  tag_id integer,
  primary key (post_id, tag_id)
);

-- migrate:down
drop table post_tags;
drop table tags;
