                                                                            
CREATE TABLE sdk_test (                                                       
    id INTEGER PRIMARY KEY,                                                   
    name TEXT,                                                                
    email TEXT,                                                               
    age INTEGER,                                                              
    status TEXT                                                               
);                                                                            

CREATE TABLE users (                                                          
    id INTEGER PRIMARY KEY,                                                   
    name TEXT,                                                                
    email TEXT,                                                               
    age INTEGER                                                               
);                                                                            
                                                                            
CREATE TABLE posts (                                                          
    id INTEGER PRIMARY KEY,                                                   
    title TEXT,                                                               
    content TEXT,                                                             
    user_id INTEGER REFERENCES users(id)                                      
);                                                                            
