package modulos

// IDsH son los módulos opcionales de producción (etapa H) en el orden en que aparecen en la app.
var IDsH = []string{IDBienvenida, IDDifusion, IDProgramados, IDInvitar, IDDescargas, IDTraductor, IDIA, IDLlamadas, IDJuegos, IDVideo, IDSorteos, IDVotaciones, IDTablon, IDExtras}

// RegistrarH agrega al registro central los módulos opcionales H1–H9. Todos nacen APAGADOS: registrar no enciende nada. La app lo llama
// al arrancar el bot (se puede repetir sin efecto: reemplaza por id).
func RegistrarH() error {
	for _, m := range []Modulo{bienvenida(), difusion(), programados(), invitar(), descargas(), traductor(), ia(), llamadas(), juegos(), descargasVideo(), sorteos(), votaciones(), tablon(), extras()} {
		if err := Registrar(m); err != nil {
			return err
		}
	}
	return nil
}

// QuitarH saca del registro los módulos H (lo usan las pruebas).
func QuitarH() {
	for _, id := range IDsH {
		Quitar(id)
	}
}
