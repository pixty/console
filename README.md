# Pixty Console 
This is a GO project which provides Pixty ReST API.

# Development environment
If you want to make changes in console you need to set-up it locally, if you just want to run the console you can use latest docker image (see [Run the console using Docker](#run-the-console-using-docker))

## Install golang (version 1.9 or greater)
## Clone pixty-console project from github.com:

```
$ mkdir $GOPATH/src/github.com/pixty
$ cd $GOPATH/src/github.com/pixty
$ git clone git@github.com:pixty/console.git
```
## Installing GRPC
You MUST use the following versions:

### Protobuf
version 3.4.0, please check:
```
$ protoc --version
libprotoc 3.4.0
```
### GRPC
version 1.4.6, for Mac users use brew. Please check:
```
$ brew list grpc
/usr/local/Cellar/grpc/1.4.6_1/bin/grpc_cli
/usr/local/Cellar/grpc/1.4.6_1/bin/grpc_cpp_plugin
/usr/local/Cellar/grpc/1.4.6_1/bin/grpc_csharp_plugin
/usr/local/Cellar/grpc/1.4.6_1/bin/grpc_node_plugin
/usr/local/Cellar/grpc/1.4.6_1/bin/grpc_objective_c_plugin
...
```

### GRPC go version
go get will bring you latest master which doesn't work, so you need to go to ${GOPATH}/src/google.golang.org/grpc
and use the version 1.5.2:
```
$ git checkout v1.5.2
Previous HEAD position was f92cdcd... Change version to 1.6.0
HEAD is now at b3ddf78... Change version to 1.5.2
```

##  Compile the console:
```
$ go get
$ go install -v ./...
```
##  Run console locally:
```
$ pixty_console -help
...
```

### Run the console using Docker (TBD. Not relevant yet)
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
$ pixty_console -debug
```
