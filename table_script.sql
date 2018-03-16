create table site_prefs (
  id int not null auto_increment,
  name varchar(100) not null,
  value text not null,
  primary key(id),
  CONSTRAINT site_prefs_value_unique UNIQUE (name)
);