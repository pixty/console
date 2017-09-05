# Pixty Console 
This is a GO project which provides Pixty ReST API.

# Development environment
If you want to make changes in console you need to set-up it locally, if you just want to run the console you can use latest docker image (see [Run the console using Docker](#run-the-console-using-docker))

- Install golang (version 1.7 or greater)
- Clone pixty-console project from github.com:

```
$ mkdir $GOPATH/src/github.com/pixty
$ cd $GOPATH/src/github.com/pixty
$ git clone git@github.com:pixty/console.git
```
-  Compile the console:
```
$ go get
$ go install -v ./...
```
-  Now you can run console locally:
```
$ pixty_console -help
...
```

# Installing locally
The instructions are about to run the console locally. You can use whether to set up all debug environment or use docker to run it.
### Run the console using Docker
 - Install Docker, if you don't have it installed on your system yet: https://www.docker.com/
 - Create new account if you don't have one on https://dockerhub.com
 - in command line type:
 
 ```
 $ docker login
 ```
 
 - As soon as you get to be logged into `dockerhub.com` run the command:
 
 ```
 $ docker run -p 8080:8080 -it dspasibenko/pixty-console
 ```
 
 - To test that console runs and up:
 
 ```
 $ curl localhost:8080/ping
pong
 ```
 
The command above will run console in container listening on port 8080, which is mapped to the same one (8080) on the host machine. If you would like to use another port (let's say 9999) just change the `-p` parameter to `-p 9999:8080`, which will allow to reach the console using port 9999 on the host machine.
 
### Run the console using development environment
If you would like to run the console locally please do the following:

1. Install the [Development environment](#development-environment)
2. Run the console:

```
$ console -debug
```
