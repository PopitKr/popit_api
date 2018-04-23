create table site_prefs (
  id int not null auto_increment,
  name varchar(100) not null,
  value text not null,
  primary key(id),
  CONSTRAINT site_prefs_value_unique UNIQUE (name)
);

create table post_external_metas (
  id int not null auto_increment,
  post_id int not null ,
  name varchar(100) not null,
  value varchar(100) not null,
  primary key(id)
);

insert into post_external_metas (post_id, name, value)
select ID, 'facebook.like.http', 'true'
from wprdh0703_posts
where post_status = 'publish'
and post_type = 'post';

update post_external_metas set value = 'false' where post_id = 16506 and name = 'facebook.like.http';