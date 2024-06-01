package utils

import (
  "fmt"
  )

func PanicOn(err error, msg string) {
  if err != nil {
    fmt.Println(err)
    panic(msg)
  }
  }

func LogOn(err error, msg string) (e error) {
  if err != nil {
    fmt.Println(err)
    fmt.Println(msg)
    e = err
  }
  return
  }

func Log(err interface{}){
  fmt.Println(err)
  }

func Println(sth interface{}){
  fmt.Println(sth)
  }









