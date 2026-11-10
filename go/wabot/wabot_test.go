package wabot

import (
	"sync"
	"testing"
)

type falso struct {
	mu      sync.Mutex
	estados []string
	logs    int
}

func (f *falso) OnStatus(t string)      { f.mu.Lock(); f.estados = append(f.estados, t); f.mu.Unlock() }
func (f *falso) OnPairingCode(c string) {}
func (f *falso) OnLog(l string)         { f.mu.Lock(); f.logs++; f.mu.Unlock() }

// Sin conexión real: abre la base, valida números y cierra.
func TestStartPairStop(t *testing.T) {
	f := &falso{}
	if err := Start(t.TempDir(), f); err != nil {
		t.Fatal(err)
	}
	if IsConnected() {
		t.Fatal("no debe estar conectado")
	}
	if err := Start(t.TempDir(), f); err == nil {
		t.Fatal("un segundo Start debía fallar")
	}
	if err := Pair("12345"); err == nil {
		t.Fatal("número corto debía fallar")
	}
	if err := Pair("0123456789"); err == nil {
		t.Fatal("número sin código de país debía fallar")
	}
	Stop()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.estados) < 2 || f.estados[0] != "sin_vincular" || f.estados[len(f.estados)-1] != "detenido" {
		t.Fatalf("estados inesperados: %v", f.estados)
	}
	if err := Pair("525512345678"); err == nil {
		t.Fatal("después de Stop, Pair debía fallar")
	}
}
