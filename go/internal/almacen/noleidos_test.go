package almacen

import (
	"strings"
	"testing"
)

func TestContarNoLeidosEnChatGrande(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, "5215500000001@s.whatsapp.net", "5215500000001", "Ana", false)
	chatBasico(t, a, "5215500000002@s.whatsapp.net", "5215500000002", "Beto", false)
	const chat, otro = "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	// 4 000 mensajes: de cada 3, uno es mío. ts = 1000 + i.
	esperados := 0
	for i := 0; i < 4000; i++ {
		mio := 0
		if i%3 == 0 {
			mio = 1
		}
		if mio == 0 && 1000+i > 1000+1500 {
			esperados++
		}
		if _, err := tx.Exec(`INSERT INTO messages(id, chat_jid, bot_id, es_mio, ts) VALUES(?,?,1,?,?)`, "m"+itoa(i), chat, mio, 1000+i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO messages(id, chat_jid, bot_id, es_mio, ts) VALUES('o1', ?, 1, 0, 99999)`, otro); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	n, err := a.ContarNoLeidos(1, chat, 1000+1500)
	if err != nil || n != esperados {
		t.Fatalf("no leídos %d (esperaba %d) err=%v", n, esperados, err)
	}
	if n <= 40 {
		t.Fatalf("el conteo real debe pasar de la primera página de 40: %d", n)
	}
	if n, _ := a.ContarNoLeidos(1, chat, 1000+4000); n != 0 {
		t.Fatalf("nada posterior: %d", n)
	}
	if n, _ := a.ContarNoLeidos(1, otro, 0); n != 1 {
		t.Fatalf("otro chat: %d", n)
	}
	// Usa el índice parcial (y no recorre el chat).
	var plan strings.Builder
	filas, err := a.db.Query(`EXPLAIN QUERY PLAN SELECT COUNT(*) FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND ts>?`, 1, chat, 5)
	if err != nil {
		t.Fatal(err)
	}
	for filas.Next() {
		var a, b, c int
		var det string
		_ = filas.Scan(&a, &b, &c, &det)
		plan.WriteString(det)
	}
	filas.Close()
	if !strings.Contains(plan.String(), "idx_mensajes_noleidos") {
		t.Fatalf("no usa el índice de no leídos: %s", plan.String())
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
