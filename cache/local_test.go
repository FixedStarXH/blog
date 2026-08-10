package cache

import (
	"testing"
	"time"
)

type localTestObj struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func TestLocalSetGet(t *testing.T) {
	key := "local:test:get"
	SetLocal(key, localTestObj{ID: 1, Name: "hello"}, time.Minute)
	var out localTestObj
	if !GetLocal(key, &out) {
		t.Fatal("写入后应立即命中")
	}
	if out.Name != "hello" {
		t.Fatalf("期望 Name=hello，实际 %s", out.Name)
	}
}

func TestLocalMiss(t *testing.T) {
	var out localTestObj
	if GetLocal("local:test:not-exist", &out) {
		t.Fatal("不存在的 key 不应命中")
	}
}

func TestLocalDel(t *testing.T) {
	key := "local:test:del"
	SetLocal(key, localTestObj{ID: 2}, time.Minute)
	DelLocal(key)
	var out localTestObj
	if GetLocal(key, &out) {
		t.Fatal("DelLocal 后不应再命中")
	}
}

func TestLocalExpire(t *testing.T) {
	key := "local:test:expire"
	SetLocal(key, localTestObj{ID: 3}, time.Second)
	var out localTestObj
	if !GetLocal(key, &out) {
		t.Fatal("写入后应立即命中")
	}
	time.Sleep(1200 * time.Millisecond)
	if GetLocal(key, &out) {
		t.Fatal("TTL 过期后不应命中")
	}
}
