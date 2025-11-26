package discord

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const (
	mapGalleryUrl = "https://mapgallery.realitymod.com"
)

//go:embed assets/levels.json
var levelsJson []byte

type galleryMap struct {
	Name string `json:"Name"`
	Key  string `json:"Key"`
	// Resolution int    `json:"Resolution"`
	Size int `json:"Size"`
	// Color      string `json:"Color"`
}

func (m galleryMap) FullName() string {
	return m.Name + " (" + strconv.Itoa(m.Size) + " km)"
}

type mapGallery struct {
	httpClient *http.Client

	maps map[string]galleryMap // galleryMap.Key -> galleryMap
}

func newMapGallery(httpClient *http.Client) (*mapGallery, error) {
	mc := &mapGallery{
		httpClient: httpClient,
		maps:       make(map[string]galleryMap),
	}

	err := mc.fetchMaps()
	if err != nil {
		var maps []galleryMap
		err := json.Unmarshal(levelsJson, &maps)
		if err != nil {
			return nil, err
		}

		for _, m := range maps {
			mc.maps[m.Key] = m
		}
	}

	return mc, nil
}

func (mg *mapGallery) fetchMaps() error {
	resp, err := mg.httpClient.Get(mapGalleryUrl + "json/levels.json")
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch maps: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var maps []galleryMap
	err = json.Unmarshal(body, &maps)
	if err != nil {
		return err
	}

	for _, m := range maps {
		mg.maps[m.Key] = m
	}

	return nil
}

func (mg *mapGallery) GetMapByKey(key string) (galleryMap, bool) {
	m, ok := mg.maps[key]
	if !ok {
		return galleryMap{
			Name: key,
			Key:  key,
			Size: 0,
		}, false
	}

	return m, true
}

func (mg *mapGallery) FetchMapTile(mapName string) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/images/maps/%s/tile.jpg", mapGalleryUrl, getKey(mapName))

	resp, err := mg.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to fetch map tile: status code %d", resp.StatusCode)
	}

	return resp.Body, nil
}
