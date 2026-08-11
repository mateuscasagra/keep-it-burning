// Command genicon desenha o ícone do aplicativo — a mesma fogueira da
// interface — e grava o arquivo icon.ico usado nos recursos do executável.
//
// O ícone é gerado por código, e não guardado como imagem editada à mão, para
// que continue igual à fogueira do app se as cores mudarem. Rode a partir da
// raiz do projeto:
//
//	go run ./tools/genicon
//
// Depois disso, regenere os recursos do Windows conforme o README.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"

	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// Cores do ícone, alinhadas com a paleta da fogueira em internal/ui/fire.go.
var (
	flameOuter = color.NRGBA{R: 0xe0, G: 0x30, B: 0x14, A: 0xff}
	flameMid   = color.NRGBA{R: 0xf9, G: 0x7c, B: 0x0e, A: 0xff}
	flameInner = color.NRGBA{R: 0xfd, G: 0xd0, B: 0x35, A: 0xff}
	logLight   = color.NRGBA{R: 0xb4, G: 0x80, B: 0x50, A: 0xff}
	logDark    = color.NRGBA{R: 0x6d, G: 0x4a, B: 0x2f, A: 0xff}
	logBurnt   = color.NRGBA{R: 0x3a, G: 0x31, B: 0x2b, A: 0xff}
)

// iconSizes são os tamanhos que entram no .ico. O Windows escolhe o mais
// adequado para cada contexto: 16 na barra de título, 32 na barra de tarefas,
// 256 na visualização em ícones extra grandes do Explorer.
var iconSizes = []int{16, 20, 24, 32, 48, 64, 128, 256}

// render é a resolução em que tudo é desenhado antes de reduzir. Desenhar
// grande e reduzir com filtro dá bordas muito melhores do que rasterizar
// direto em 16 pixels.
const render = 1024

func main() {
	out := flag.String("o", "icon.ico", "arquivo .ico de saída")
	pngDir := flag.String("png", "", "se informado, também grava os PNGs neste diretório")
	flag.Parse()

	master := drawIcon(render)

	frames := make([]*image.NRGBA, 0, len(iconSizes))
	for _, size := range iconSizes {
		frames = append(frames, resize(master, size))
	}

	if *pngDir != "" {
		if err := os.MkdirAll(*pngDir, 0o755); err != nil {
			log.Fatalf("genicon: %v", err)
		}
		for i, size := range iconSizes {
			name := filepath.Join(*pngDir, fmt.Sprintf("icon-%d.png", size))
			if err := writePNG(name, frames[i]); err != nil {
				log.Fatalf("genicon: %v", err)
			}
		}
	}

	if err := writeICO(*out, frames); err != nil {
		log.Fatalf("genicon: %v", err)
	}
	fmt.Printf("genicon: %s gravado com %d tamanhos\n", *out, len(frames))
}

// drawIcon desenha a fogueira num quadrado de lado size, com fundo
// transparente.
func drawIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float32(size)

	// Chamas laterais primeiro, para o miolo ficar por cima delas.
	fillPath(img, flameOuter, flamePath(s, 0.272, 0.845, 0.32, 0.44, 0.022))
	fillPath(img, flameOuter, flamePath(s, 0.728, 0.845, 0.31, 0.42, -0.022))

	// Chama central: a mais alta, em três camadas concêntricas.
	fillPath(img, flameOuter, flamePath(s, 0.500, 0.870, 0.50, 0.78, 0.0))
	fillPath(img, flameMid, flamePath(s, 0.500, 0.870, 0.32, 0.58, 0.0))
	fillPath(img, flameInner, flamePath(s, 0.500, 0.870, 0.16, 0.37, 0.0))

	// Duas toras cruzadas na base. É a silhueta que ainda se reconhece como
	// fogueira num ícone de 16 pixels; achas verticais viravam um borrão
	// escuro no meio da chama.
	fillPath(img, logDark, capsule(s, 0.500, 0.858, 0.62, 0.115, -0.21))
	fillPath(img, logLight, capsule(s, 0.500, 0.862, 0.60, 0.110, 0.21))

	return img
}

// pathFunc adiciona um contorno ao rasterizador.
type pathFunc func(r *vector.Rasterizer)

// fillPath rasteriza um contorno e o compõe sobre a imagem.
func fillPath(dst *image.NRGBA, c color.NRGBA, p pathFunc) {
	b := dst.Bounds()
	r := vector.NewRasterizer(b.Dx(), b.Dy())
	p(r)
	r.Draw(dst, b, image.NewUniform(c), image.Point{})
}

// flamePath devolve a silhueta de uma língua de fogo. Os parâmetros são
// frações do lado do ícone: cx e base posicionam a base, w e h dão as
// dimensões e lean inclina a ponta para um dos lados.
func flamePath(s, cx, base, w, h, lean float32) pathFunc {
	return func(r *vector.Rasterizer) {
		x, y := cx*s, base*s
		half, ht := w*s/2, h*s
		tip := lean * s

		r.MoveTo(x-half, y)
		// Sobe pela esquerda, estrangula na cintura e curva até a ponta. É a
		// cintura que faz a chama parecer chama, e não um triângulo.
		r.CubeTo(
			x-half*1.14, y-ht*0.14,
			x-half*1.08, y-ht*0.44,
			x-half*0.36+tip*0.4, y-ht*0.62,
		)
		r.CubeTo(
			x-half*0.16+tip*0.8, y-ht*0.80,
			x-half*0.02+tip, y-ht*0.93,
			x+tip, y-ht,
		)
		r.CubeTo(
			x+half*0.06+tip, y-ht*0.91,
			x+half*0.26+tip*0.8, y-ht*0.78,
			x+half*0.38+tip*0.4, y-ht*0.60,
		)
		r.CubeTo(
			x+half*1.08, y-ht*0.42,
			x+half*1.14, y-ht*0.14,
			x+half, y,
		)
		r.ClosePath()
	}
}

// capsule devolve o contorno de uma tora: um retângulo girado com as duas
// pontas arredondadas.
func capsule(s, cx, cy, length, thick, angle float32) pathFunc {
	return func(r *vector.Rasterizer) {
		x, y := cx*s, cy*s
		half := length * s / 2
		rad := thick * s / 2
		sin, cos := float32(math.Sin(float64(angle))), float32(math.Cos(float64(angle)))

		// rot leva um ponto do referencial da tora (deitada na horizontal)
		// para o referencial da imagem.
		rot := func(px, py float32) (float32, float32) {
			return x + px*cos - py*sin, y + px*sin + py*cos
		}
		// k é a constante que aproxima um quarto de círculo por uma cúbica.
		const k = 0.5523

		p := func(px, py float32) (float32, float32) { return rot(px, py) }

		ax, ay := p(-half, -rad)
		r.MoveTo(ax, ay)

		bx, by := p(half, -rad)
		r.LineTo(bx, by)

		// Ponta direita.
		c1x, c1y := p(half+rad*k, -rad)
		c2x, c2y := p(half+rad, -rad*k)
		ex, ey := p(half+rad, 0)
		r.CubeTo(c1x, c1y, c2x, c2y, ex, ey)
		c1x, c1y = p(half+rad, rad*k)
		c2x, c2y = p(half+rad*k, rad)
		ex, ey = p(half, rad)
		r.CubeTo(c1x, c1y, c2x, c2y, ex, ey)

		ex, ey = p(-half, rad)
		r.LineTo(ex, ey)

		// Ponta esquerda.
		c1x, c1y = p(-half-rad*k, rad)
		c2x, c2y = p(-half-rad, rad*k)
		ex, ey = p(-half-rad, 0)
		r.CubeTo(c1x, c1y, c2x, c2y, ex, ey)
		c1x, c1y = p(-half-rad, -rad*k)
		c2x, c2y = p(-half-rad*k, -rad)
		ex, ey = p(-half, -rad)
		r.CubeTo(c1x, c1y, c2x, c2y, ex, ey)

		r.ClosePath()
	}
}

// resize reduz a imagem mestre para um tamanho do ícone.
func resize(src *image.NRGBA, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	return f.Close()
}

// Cabeçalhos do formato ICO, conforme a documentação do Windows.
type icoHeader struct {
	Reserved uint16 // sempre 0
	Type     uint16 // 1 = ícone
	Count    uint16
}

type icoEntry struct {
	Width    uint8 // 0 significa 256
	Height   uint8 // 0 significa 256
	Colors   uint8
	Reserved uint8
	Planes   uint16
	BitCount uint16
	Size     uint32
	Offset   uint32
}

// writeICO grava um .ico com uma imagem PNG por tamanho. O Windows aceita
// quadros em PNG desde o Vista, o que evita ter de montar bitmaps DIB com
// máscara de transparência à mão.
func writeICO(name string, frames []*image.NRGBA) error {
	blobs := make([][]byte, len(frames))
	for i, f := range frames {
		var buf bytes.Buffer
		if err := png.Encode(&buf, f); err != nil {
			return fmt.Errorf("codificar quadro %d: %w", i, err)
		}
		blobs[i] = buf.Bytes()
	}

	var out bytes.Buffer
	if err := binary.Write(&out, binary.LittleEndian, icoHeader{Type: 1, Count: uint16(len(frames))}); err != nil {
		return err
	}

	// Os dados começam depois do cabeçalho e de todas as entradas.
	offset := uint32(6 + 16*len(frames))
	for i, f := range frames {
		side := f.Bounds().Dx()
		dim := uint8(side)
		if side >= 256 {
			dim = 0 // 256 é representado por zero no formato
		}
		e := icoEntry{
			Width:    dim,
			Height:   dim,
			Planes:   1,
			BitCount: 32,
			Size:     uint32(len(blobs[i])),
			Offset:   offset,
		}
		if err := binary.Write(&out, binary.LittleEndian, e); err != nil {
			return err
		}
		offset += uint32(len(blobs[i]))
	}
	for _, b := range blobs {
		out.Write(b)
	}

	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(out.Bytes()); err != nil {
		return err
	}
	return f.Close()
}
