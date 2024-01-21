-- Online SQL Editor to Run SQL Online.
-- Use the editor to create new tables, insert data and all other SQL operations.
  
-- DROP TABLE Customers;
-- DROP TABLE Orders;
-- DROP TABLE Shippings;
-- DROP TABLE job_skills;
-- DROP TABLE job_roles;
-- DROP TABLE skills_tree;
-- DROP TABLE curriculum_vitae;

CREATE TABLE IF NOT EXISTS users (
  id integer primary key autoincrement,
  username varchar(100)
);
  
CREATE TABLE IF NOT EXISTS job_skills (
  id integer primary key autoincrement,
  name varchar(100)
);

CREATE TABLE IF NOT EXISTS job_roles (
  id integer primary key autoincrement,
  name varchar(100)
);

CREATE TABLE IF NOT EXISTS skills_tree (
  id integer primary key autoincrement,
  job_skill_id int,
  graduate_id int,
  UNIQUE (job_skill_id, graduate_id),
  FOREIGN KEY (graduate_id) REFERENCES users (id),
  FOREIGN KEY (job_skill_id) REFERENCES job_skills (id)
);

CREATE TABLE IF NOT EXISTS curriculum_vitae (
  id integer primary key autoincrement,
  gpa int CHECK (gpa >= 0),
  job_role_id int,
  yoe float,
  graduate_id int,
  FOREIGN KEY (job_role_id) REFERENCES job_roles (id),
  FOREIGN KEY (graduate_id) REFERENCES users (id)
);

insert into users (username) values ('tamfu'), ('steve'), ('melcore'), ('mc-tominay');
insert into job_roles (name) values ('IT Intern'), ('Cooking Chief'), ('Accountant Officer'), ('Janitor');
insert into job_skills (name) values ('C#'), ('C++'), ('Docker'), ('Unreal Engine'), ('Communication'), ('English');

insert into skills_tree (graduate_id, job_skill_id) values ('2', '1'), ('2', '3'), ('2', '6'), ('3', '5'), ('3', '6'), ('3', '4');
insert into curriculum_vitae (gpa, yoe, job_role_id, graduate_id) values ('3.5', '1.3', '1', '2'), ('2.8', '2', '3', '3');


-- select * from job_skills;

-- select * from curriculum_vitae c inner join users u on c.graduate_id = u.id inner join job_roles r on c.job_role_id = r.id;

-- select * from skills_tree t inner join job_skills s on t.job_skill_id = s.id inner join users u on t.graduate_id = u.id;

-- select t.id, u.id as user_id, u.name as username, s.name as skill_name from skills_tree t inner join job_skills s on t.job_skill_id = s.id inner join users u on t.graduate_id = u.id;

-- select * 
-- from curriculum_vitae c 
-- inner join users u on c.graduate_id = u.id 
-- inner join job_roles r on c.job_role_id = r.id
-- inner join (select t.id, u.id as user_id, u.username as username, s.name as skill_name from skills_tree t inner join job_skills s on t.job_skill_id = s.id inner join users u on t.graduate_id = u.id) t on c.graduate_id = t.user_id;
  

select c.id, c.gpa, c.yoe, r.id AS role_id, r.name AS role_name, t.* 
from curriculum_vitae c 
inner join users u on c.graduate_id = u.id 
inner join job_roles r on c.job_role_id = r.id
inner join (
  select u.id AS user_id, u.username AS user_name, s.id AS skill_id, s.name AS skill_name, t.id AS tree_id 
  from skills_tree t 
  inner join job_skills s on t.job_skill_id = s.id 
  inner join users u on t.graduate_id = u.id 
) t on c.graduate_id = t.user_id;
