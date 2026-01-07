## What is Atomicbase?

> [!IMPORTANT]  
> **Atomicbase is in very early stages of development.** It is not ready for use in projects yet.

Atomicbase is a rest api that makes managing and querying turso databases significantly easier. It works for databases that all share a schema and databases with all different schemas.

Atomicbase provides a thin abstraction over your queries and maintains a separate schema cache for each database so that queries can be made efficiently and safely.

Through the combination of parameterizing values and checking table and column names against the schema cache, no unchecked sql should ever be executed when querying a database.

This does not always make sql injection impossible though because sqlite allows for table and column names to be anything including malicious sql. This means any changes to a database's schema based on user input must be sanitized before they are executed.

## Contributing

Atomicbase is fully free and open source under the Apache-2.0 license. It is free for any kind of use.

All contributions are appreciated including:

- [Contributing to the source code](https://github.com/joe-ervin05/atomicbase/blob/main/CONTRIBUTING.MD)
- [Reporting issues and suggesting new features](https://github.com/joe-ervin05/atomicbase/issues)
