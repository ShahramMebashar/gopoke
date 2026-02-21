//go:build !(wails && darwin)

package main

func setupNativeToolbar() {}

func updateToolbarRunState(_ bool) {}
