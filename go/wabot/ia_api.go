package wabot

import "botchat/internal/modulos"

// IAProveedores devuelve (JSON) los servicios de IA que se pueden elegir, con su dirección, formato y modelos sugeridos.
func IAProveedores() string { return modulos.ProveedoresIA() }

// IAProbar hace una consulta corta con la configuración y llave actuales y devuelve lo que respondió el modelo.
func IAProbar(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return g.ProbarIA()
}
