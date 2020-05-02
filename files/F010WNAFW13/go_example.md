以下のようにファイルが配置されているとします

```
~/go/src/github.com/lifestyledesign/fabric_tokyo_hermes
├── go.mod
├── go.sum
├── hermes
│   └── dispatcher
│       └── dispatcher.go
├── idl
│   ├── hermes.pb.go
│   └── hermes.proto
└── server
    └── main.gol
```

hermes/dispatcher/dispatcher.go を以下とします

```
package dispatcher

func Hello() {
  println("hello")
}
```

idl/hermes.pb.go を以下とします

```
package idl

func World() {
  println("world")
}
```

server/main.go は以下のように書けます

```
package main

import (
  "github.com/lifestyledesign/fabric_tokyo_hermes/hermes/dispatcher"
  "github.com/lifestyledesign/fabric_tokyo_hermes/idl"
)

func main(){
  dispatcher.Hello()
  idl.World()
}
```

ただし、 go.mod は以下とします

```
module github.com/lifestyledesign/fabric_tokyo_hermes

go 1.14
```

このとき `go run server/main.go` の出力結果は以下です

```
hello
world
```
