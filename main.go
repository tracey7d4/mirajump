// Copyright 2018 The Ebiten Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math/rand"
	"time"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/audio"
	"github.com/hajimehoshi/ebiten/audio/vorbis"
	"github.com/hajimehoshi/ebiten/audio/wav"
	raudio "github.com/hajimehoshi/ebiten/examples/resources/audio"
	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	resources "github.com/hajimehoshi/ebiten/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/inpututil"
	"github.com/hajimehoshi/ebiten/text"
	localsrc "github.com/tracey7d4/mirajump/resources/images"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func floorDiv(x, y int) int {
	d := x / y
	if d*y == x || x >= 0 {
		return d
	}
	return d - 1
}

func floorMod(x, y int) int {
	return x - floorDiv(x, y)*y
}

const (
	screenWidth      = 640
	screenHeight     = 480
	tileSize         = 32
	fontSize         = 32
	smallFontSize    = fontSize / 2
	pipeWidth        = tileSize * 2
	pipeStartOffsetX = 8
	pipeIntervalX    = 10
)

var (
	miraImage       *ebiten.Image
	tilesImage      *ebiten.Image
	arcadeFont      font.Face
	smallArcadeFont font.Face
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(localsrc.Mira_png))
	if err != nil {
		log.Fatal(err)
	}
	miraImage, _ = ebiten.NewImageFromImage(img, ebiten.FilterDefault)

	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage, _ = ebiten.NewImageFromImage(img, ebiten.FilterDefault)
}

func init() {
	tt, err := truetype.Parse(fonts.ArcadeN_ttf)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	arcadeFont = truetype.NewFace(tt, &truetype.Options{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	smallArcadeFont = truetype.NewFace(tt, &truetype.Options{
		Size:    smallFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

var (
	audioContext *audio.Context
	jumpPlayer   *audio.Player
	hitPlayer    *audio.Player
)

func init() {
	audioContext, _ = audio.NewContext(44100)

	jumpD, err := vorbis.Decode(audioContext, audio.BytesReadSeekCloser(raudio.Jump_ogg))
	if err != nil {
		log.Fatal(err)
	}
	jumpPlayer, err = audio.NewPlayer(audioContext, jumpD)
	if err != nil {
		log.Fatal(err)
	}

	jabD, err := wav.Decode(audioContext, audio.BytesReadSeekCloser(raudio.Jab_wav))
	if err != nil {
		log.Fatal(err)
	}
	hitPlayer, err = audio.NewPlayer(audioContext, jabD)
	if err != nil {
		log.Fatal(err)
	}
}

type Mode int

const (
	ModeTitle Mode = iota
	ModeGame
	ModeGameOver
)
const (
	miraWidth  = 30
	miraHeight = 60
)
type Game struct {
	mode Mode

	// position
	x16  int
	y16  int
	vy16 int // gravity

	// Camera
	cameraX int
	cameraY int

	// Pipes
	pipeTileYs []int

	gameoverCount int
}

func NewGame() *Game {
	g := &Game{}
	g.init()
	return g
}

func (g *Game) init() {
	g.x16 = 0
	g.y16 = (screenHeight-tileSize)*16
	g.cameraX = -240
	g.cameraY = 0
	// height of pipe
	g.pipeTileYs = make([]int, 256)
	for i := range g.pipeTileYs {
		g.pipeTileYs[i] = 12
	}
}

func jump() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if len(inpututil.JustPressedTouchIDs()) > 0 {
		return true
	}
	return false
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) Update(screen *ebiten.Image) error {
	switch g.mode {
	case ModeTitle:
		if jump() {
			g.mode = ModeGame
		}
	case ModeGame:
		g.x16 += 32 // horizontal speed of gopher is static 32 units/tick
		g.cameraX += 2
		_, h := miraImage.Size()
		if jump() && g.y16 == (screenHeight-tileSize)*16 -8*(h+miraHeight)-200 {
			g.vy16 = -120        // jump up 96 units
			jumpPlayer.Rewind() // play jump sound
			jumpPlayer.Play()
		}
		g.y16 += g.vy16

		// Gravity
		g.vy16 += 3
		if g.vy16 > 120 {
			g.vy16 = 120
		}

		// run on the ground
		if g.y16 > (screenHeight-tileSize)*16 -8*(h+miraHeight)-200{
			g.y16 = (screenHeight-tileSize)*16 -8*(h+miraHeight)-200
		}

		// if hit then game over
		if g.hit() {
			hitPlayer.Rewind()
			hitPlayer.Play()
			g.mode = ModeGameOver
			g.gameoverCount = 30
		}
	case ModeGameOver:
		if g.gameoverCount > 0 {
			g.gameoverCount--
		}
		if g.gameoverCount == 0 && jump() {
			g.init()
			g.mode = ModeTitle
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x80, 0xa0, 0xc0, 0xff})
	g.drawTiles(screen)
	if g.mode != ModeTitle {
		g.drawMiraImage(screen)
	}
	var texts []string
	switch g.mode {
	case ModeTitle:
		texts = []string{"MIRA JUMP", "", "", "", "", "PRESS SPACE KEY", "", "OR TOUCH SCREEN"}
	case ModeGameOver:
		texts = []string{"", "GAME OVER!"}
	}
	for i, l := range texts {
		x := (screenWidth - len(l)*fontSize) / 2
		text.Draw(screen, l, arcadeFont, x, (i+4)*fontSize, color.White)
	}

	if g.mode == ModeTitle {
		msg := []string{"Modified by TraceyH"}
		for i, l := range msg {
			x := (screenWidth - len(l)*smallFontSize) / 2
			text.Draw(screen, l, smallArcadeFont, x, screenHeight-4+(i-1)*smallFontSize, color.Black)
		}
	}

	scoreStr := fmt.Sprintf("%04d", g.score())
	text.Draw(screen, scoreStr, arcadeFont, screenWidth-len(scoreStr)*fontSize, fontSize, color.White)
	//ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS()))
}

func (g *Game) pipeAt(tileX int) (tileY int, ok bool) {
	if (tileX - pipeStartOffsetX) <= 0 {
		return 0, false
	}
	if floorMod(tileX-pipeStartOffsetX, pipeIntervalX) != 0 {
		return 0, false
	}
	idx := floorDiv(tileX-pipeStartOffsetX, pipeIntervalX)
	return g.pipeTileYs[idx%len(g.pipeTileYs)], true
}

func (g *Game) score() int {
	x := floorDiv(g.x16, 16) / tileSize
	if (x - pipeStartOffsetX) <= 0 {
		return 0
	}
	return floorDiv(x-pipeStartOffsetX, pipeIntervalX)
}

func (g *Game) hit() bool {
	if g.mode != ModeGame {
		return false
	}

	w, h := miraImage.Size()
	x0 := floorDiv(g.x16, 16) + (w-miraWidth)/2
	y0 := floorDiv(g.y16, 16) + (h-miraHeight)/2
	x1 := x0 + miraWidth
	y1 := y0 + miraHeight
	//if y0 < -tileSize*4 {
	//	return true
	//}

	xMin := floorDiv(x0-pipeWidth, tileSize)
	xMax := floorDiv(x0+miraWidth, tileSize)
	for x := xMin; x <= xMax; x++ {
		y, ok := g.pipeAt(x)
		if !ok {
			continue
		}
		if x0 >= x*tileSize+pipeWidth {
			continue
		}
		if x1 < x*tileSize {
			continue
		}
		// check whether hit pipe
		if y1 >= y*tileSize -240/16 {
			return true
		}
	}
	return false
}

func (g *Game) drawTiles(screen *ebiten.Image) {
	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	op := &ebiten.DrawImageOptions{}
	for i := -2; i < nx+1; i++ {
		// ground
		op.GeoM.Reset()
		op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
			float64((ny-1)*tileSize-floorMod(g.cameraY, tileSize)))
		screen.DrawImage(tilesImage.SubImage(image.Rect(0, 0, tileSize, tileSize)).(*ebiten.Image), op)

		// pipe
		if tileY, ok := g.pipeAt(floorDiv(g.cameraX, tileSize) + i); ok {
			// lower pipes
			for j := tileY; j < screenHeight/tileSize-1; j++ {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				var r image.Rectangle
				if j == tileY {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize+tileSize)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
		}
	}
}

func (g *Game) drawMiraImage(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	w, h := miraImage.Size()
	op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
	//op.GeoM.Scale(0.5, 0.5)
	//op.GeoM.Rotate(float64(g.vy16) / 96.0 * math.Pi / 32)
	op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	op.GeoM.Translate(float64(g.x16/16.0)-float64(g.cameraX), float64(g.y16/16.0)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(miraImage, op)
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Mira Jump")
	if err := ebiten.RunGame(NewGame()); err != nil {
		panic(err)
	}
}
