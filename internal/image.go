package internal

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
)

const (
	width  = 400
	height = 120

	flagWidth  = 50
	flagHeight = 28

	cacheWH = 32

	gpmInsurgency = "gpm_insurgency"
	gpmGungame    = "gpm_gungame"

	fontTypeBold         = "Bold"
	fontTypeMediumItalic = "MediumItalic"
)

//go:embed assets/*
var assets embed.FS

func createImage(summary *jsonSummary) (io.Reader, error) {
	mapName := factionsLayersModes.MapNames[summary.MapName]

	dc := gg.NewContext(width, height)

	bgImg, err := loadImageFromURL(mapName.ImageUrl, summary.MapName)
	if err != nil {
		return nil, err
	}

	drawScaledImage(dc, bgImg, 0, 0, width, height)

	templateImage, err := loadImage("template.png")
	if err != nil {
		return nil, err
	}

	dc.DrawImage(templateImage, 0, 0)

	err = setFont(dc, 24, fontTypeBold)
	dc.SetRGB(1, 1, 1)
	dc.DrawStringAnchored(factionsLayersModes.MapNames[summary.MapName].Name, 200, 15, 0.5, 1)

	setFont(dc, 17, fontTypeMediumItalic)
	layerMode := fmt.Sprintf("%s, %s", factionsLayersModes.GameModes[summary.MapMode].Name, factionsLayersModes.Layers[summary.MapLayer].Name)
	dc.DrawStringAnchored(layerMode, 200, 48, 0.5, 0.5)

	if summary.MapMode == gpmGungame {
		err = drawGGWinner(dc, findGGWinner(summary.Players))
		if err != nil {
			return nil, err
		}
	} else {
		err = drawTickets(dc, summary)
		if err != nil {
			return nil, err
		}
	}

	out := &bytes.Buffer{}

	err = png.Encode(out, dc.Image())
	if err != nil {
		return nil, err
	}

	return out, nil
}

func drawGGWinner(dc *gg.Context, winner string) error {
	err := setFont(dc, 13, fontTypeBold)
	if err != nil {
		return err
	}

	dc.DrawStringAnchored("Winner:", 200, 70, 0.5, 0.5)

	err = setFont(dc, 19, fontTypeBold)
	if err != nil {
		return err
	}
	dc.DrawStringAnchored(winner, 200, 87, 0.5, 0.5)

	return nil
}

func findGGWinner(players []player) string {
	var winner player

	for _, p := range players {
		if p.Score > winner.Score {
			winner = p
		}
	}

	return winner.Name
}

func drawTickets(dc *gg.Context, summary *jsonSummary) error {
	setFont(dc, 34, fontTypeBold)
	dc.DrawStringAnchored(strconv.Itoa(summary.Team2Tickets), 161, 62, 0.5, 1)
	dc.DrawStringAnchored(strconv.Itoa(summary.Team1Tickets), 239, 62, 0.5, 1)

	// Team 1 flag
	flag1Img, err := loadImage(summary.Team1Name + ".png")
	if err != nil {
		return err
	}

	drawScaledImage(dc, flag1Img, 280, 70, flagWidth, flagHeight)

	flag2Img, err := loadImage(summary.Team2Name + ".png")
	if err != nil {
		return err
	}

	drawScaledImage(dc, flag2Img, 71, 70, flagWidth, flagHeight)

	if summary.MapMode == gpmInsurgency {
		cacheImg, err := loadImage("Cache.png")
		if err != nil {
			return err
		}

		drawScaledImage(dc, cacheImg, 249, 68, cacheWH, cacheWH)
	}

	return nil
}

func loadImage(filename string) (image.Image, error) {
	file, err := assets.Open(path.Join("assets", filename))
	if err != nil {
		return nil, err
	}

	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func loadImageFromURL(url, cacheID string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func setFont(dc *gg.Context, size float64, typ string) error {
	fontBytes, err := assets.ReadFile(path.Join("assets", fmt.Sprintf("OpenSans-%s.ttf", typ)))
	f, err := truetype.Parse(fontBytes)
	if err != nil {
		return err
	}

	face := truetype.NewFace(f, &truetype.Options{
		Size: size,
	})

	dc.SetFontFace(face)
	return nil
}

func drawScaledImage(dc *gg.Context, img image.Image, x, y, w, h float64) {
	scaleX := w / float64(img.Bounds().Dx())
	scaleY := h / float64(img.Bounds().Dy())

	dc.Push()
	dc.Scale(scaleX, scaleY)
	dc.DrawImageAnchored(img, int(x/scaleX), int(y/scaleY), 0, 0)
	dc.Pop()
}
