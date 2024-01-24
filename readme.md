# Graduate Backend (for Now)

## Next Task 

- [ ] Update job_skills_tree insertion and selection from API
- [ ] Manage job skills insertion from GORM (optional)

- [x] Enable CORS on server
- [ ] Compile server for Windows
- [ ] Allow static file distribution
- [ ] API documentation

## Issues With mixing 2 DB packages (GORM & database/sql) when creating tables

Although GORM is alright for simple query, this is not the case for complex one.
Instinctively, I created the table on which I was suppose to make those complex query with the standard lib.
However, that's where the issue start. You see, GORM can only understand Foreign Key when he is the one that created the references table to begin with.
Thus if table A references Table B, but table B was not created by GORM, then GORM is unable to make that references when time come for database migration **(db.AutoMigrate(&A{}) do not work create the table A anymore)**
So far, I only found that issue with table creation (db migration) between GORM and database/sql specifically.
I think that query wise, it is Okay (However, I haven't verified that claim)

