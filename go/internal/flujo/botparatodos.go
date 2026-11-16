package flujo

import (
	"botchat/internal/estilo"
)

// NombreBotParaTodos es el nombre del menú de fábrica.
const NombreBotParaTodos = "Bot Chat · menú principal"

// PlantillaBotParaTodos es el menú del «Bot para todos»: un menú principal con las categorías de lo que la gente más usa (stickers,
// descargas, IA, utilidades, juegos y notas) y dos submenús. Con el formato «Botones» (solo en chats privados) sale como botones o lista;
// en grupos y con texto sale numerado. Se abre SOLO con el símbolo de los comandos («!menu») o mencionando al bot en un grupo: nunca por
// una palabra suelta.
func PlantillaBotParaTodos() *Def {
	pie := "Escribe !menu para volver"
	resp := func(emoji, titulo string, lineas ...string) string {
		return estilo.Tarjeta(emoji, titulo, lineas, pie)
	}
	d := &Def{V: 1, Nombre: NombreBotParaTodos, Inicio: "m",
		Pasos: []Paso{
			{ID: "m", Tipo: PasoMenu, Avatar: "menu", Titulo: "🤖 *¿Qué quieres hacer, {nombre}?*\nElige una opción 👇", Opciones: []Opcion{
				{Texto: "🎨 Stickers y fotos", Destino: "r_stickers"},
				{Texto: "⬇️ Descargar videos", Destino: "r_descargar"},
				{Texto: "🧠 Preguntarle a la IA", Destino: "r_ia"},
				{Texto: "🧰 Utilidades", Destino: "u"},
				{Texto: "🎮 Juegos y diversión", Destino: "j"},
				{Texto: "📝 Notas y pendientes", Destino: "r_notas"},
				{Texto: "❓ Ayuda", Destino: "r_ayuda"},
			}},
			{ID: "u", Tipo: PasoMenu, Titulo: "🧰 *Utilidades*\n¿Cuál necesitas?", Opciones: []Opcion{
				{Texto: "🌤️ Clima", Destino: "r_clima"},
				{Texto: "🧮 Calculadora", Destino: "r_calc"},
				{Texto: "🕒 Hora en el mundo", Destino: "r_hora"},
				{Texto: "💵 Dólar", Destino: "r_dolar"},
				{Texto: "📚 Qué significa algo", Destino: "r_definir"},
				{Texto: "🔗 Acortar y QR", Destino: "r_enlaces"},
			}},
			{ID: "j", Tipo: PasoMenu, Titulo: "🎮 *Juegos y diversión*\n¿Qué se te antoja?", Opciones: []Opcion{
				{Texto: "🎲 Juegos", Destino: "r_juegos"},
				{Texto: "😄 Chiste, dato o frase", Destino: "r_diversion"},
				{Texto: "🏆 Marcador", Destino: "r_marcador"},
			}},
			{ID: "r_stickers", Tipo: PasoRespuesta, Texto: resp("🎨", "Stickers y fotos",
				"• Responde a una foto con *!sticker* y te la devuelvo como sticker.",
				"• O manda la foto con *!sticker* en el pie de foto.",
				"• *!qr texto* arma un código QR.")},
			{ID: "r_descargar", Tipo: PasoRespuesta, Texto: resp("⬇️", "Descargar videos y audios",
				"• *!descargar enlace* baja un video (YouTube, TikTok, Instagram, X…).",
				"• *!audio enlace* te manda solo el audio.",
				"_Si no responde, el dueño todavía no activa las descargas._")},
			{ID: "r_ia", Tipo: PasoRespuesta, Texto: resp("🧠", "Pregúntale a la IA",
				"• *!ia ¿por qué el cielo es azul?*",
				"• Responde a un mensaje con *!ia explica esto*.",
				"• *!traducir inglés* sobre un mensaje lo traduce.",
				"_Si no responde, el dueño todavía no conecta la IA._")},
			{ID: "r_notas", Tipo: PasoRespuesta, Texto: resp("📝", "Notas y pendientes",
				"• *!anotar comprar leche* guarda una nota; *!notas* las muestra.",
				"• *!tarea pagar la luz* agrega un pendiente; *!tareas* los muestra.",
				"• *!hecho 1* marca el pendiente 1 como hecho.")},
			{ID: "r_ayuda", Tipo: PasoRespuesta, Texto: resp("❓", "Ayuda",
				"• *!ayuda* muestra todo lo que puedes usar.",
				"• *!ayuda sticker* explica un comando con un ejemplo.",
				"• *!reglas* muestra las reglas del grupo.",
				"• *!rango* dice cuál es tu rango.")},
			{ID: "r_clima", Tipo: PasoRespuesta, Texto: resp("🌤️", "Clima", "• *!clima Monterrey* te dice cómo está el tiempo hoy.", "• Sin ciudad usa la que dejó configurada el dueño.")},
			{ID: "r_calc", Tipo: PasoRespuesta, Texto: resp("🧮", "Calculadora", "• *!calc 12*(3+4)* resuelve la cuenta.", "• Entiende + − × ÷ ^ % paréntesis, pi y sqrt().")},
			{ID: "r_hora", Tipo: PasoRespuesta, Texto: resp("🕒", "Hora en el mundo", "• *!hora* muestra la hora aquí y en otras ciudades.", "• *!hora Tokio* o *!hora Bogotá* la de esa ciudad.")},
			{ID: "r_dolar", Tipo: PasoRespuesta, Texto: resp("💵", "Dólar", "• *!dolar* muestra cuánto vale hoy.", "• *!dolar 100* convierte 100 dólares a pesos y euros.")},
			{ID: "r_definir", Tipo: PasoRespuesta, Texto: resp("📚", "Qué significa algo", "• *!definir fotosíntesis* te explica el tema (de Wikipedia).")},
			{ID: "r_enlaces", Tipo: PasoRespuesta, Texto: resp("🔗", "Acortar y QR", "• *!acortar https://…* te da un enlace corto.", "• *!qr https://…* te manda el código QR en una imagen.")},
			{ID: "r_juegos", Tipo: PasoRespuesta, Texto: resp("🎲", "Juegos", "• *!juegos* muestra los disponibles: dado, piedra papel o tijera, ruleta, adivina el número, tres en raya y trivia.", "• *!rendirse* termina tu partida.")},
			{ID: "r_diversion", Tipo: PasoRespuesta, Texto: resp("😄", "Para pasar el rato", "• *!chiste* un chiste.", "• *!dato* un dato curioso.", "• *!frase* una frase para el día.")},
			{ID: "r_marcador", Tipo: PasoRespuesta, Texto: resp("🏆", "Marcador", "• *!marcador* muestra quién va ganando en este chat.")},
		},
		Disparadores: Disparadores{Comando: ".menu", Mencion: true},
		Alcance:      Alcance{Privados: true, TodosGrupos: true},
	}
	d.Ajustes = &AjustesMenu{MaxSesionesGrupo: 5}
	return d
}
