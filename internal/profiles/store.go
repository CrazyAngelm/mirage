package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mirage/internal/bundle"
)

type Store struct {
	stateDir string
}

type File struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

type Profile struct {
	ID                     string `json:"id"`
	DisplayName            string `json:"display_name"`
	Link                   string `json:"link"`
	CamouflageSiteOverride string `json:"camouflage_site_override,omitempty"`
	LastLatencyMS          int    `json:"last_latency_ms,omitempty"`
	LastStatus             string `json:"last_status,omitempty"`
}

func NewStore(stateDir string) Store {
	return Store{stateDir: stateDir}
}

func (s Store) ImportLink(link string) error {
	link = strings.TrimSpace(strings.TrimPrefix(link, string(rune(0xfeff))))
	b, err := bundle.Decode(link)
	if err != nil {
		return err
	}
	id := b.ProfileID
	if id == "" {
		id = b.ServerHost()
	}
	if id == "" {
		return fmt.Errorf("profile id or server host required")
	}
	file, err := s.read()
	if err != nil {
		return err
	}
	profile := Profile{ID: id, DisplayName: b.Name(), Link: link}
	updated := false
	for i := range file.Profiles {
		if file.Profiles[i].ID == id {
			profile.CamouflageSiteOverride = file.Profiles[i].CamouflageSiteOverride
			profile.LastLatencyMS = file.Profiles[i].LastLatencyMS
			profile.LastStatus = file.Profiles[i].LastStatus
			file.Profiles[i] = profile
			updated = true
		}
	}
	if !updated {
		file.Profiles = append(file.Profiles, profile)
	}
	if err := s.write(file); err != nil {
		return err
	}
	if _, err := os.Stat(s.activePath()); os.IsNotExist(err) {
		return s.SetActive(id)
	}
	return nil
}

func (s Store) List() ([]Profile, error) {
	file, err := s.read()
	if err != nil {
		return nil, err
	}
	return file.Profiles, nil
}

func (s Store) Active() (Profile, error) {
	idBytes, err := os.ReadFile(s.activePath())
	if err != nil {
		return Profile{}, err
	}
	id := strings.TrimSpace(string(idBytes))
	file, err := s.read()
	if err != nil {
		return Profile{}, err
	}
	for _, profile := range file.Profiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("active profile %q not found", id)
}

func (s Store) SetActive(id string) error {
	file, err := s.read()
	if err != nil {
		return err
	}
	for _, profile := range file.Profiles {
		if profile.ID == id {
			if err := os.MkdirAll(s.stateDir, 0755); err != nil {
				return err
			}
			return os.WriteFile(s.activePath(), []byte(id+"\n"), 0600)
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

func (s Store) Remove(id string) error {
	activeID := ""
	if payload, err := os.ReadFile(s.activePath()); err == nil {
		activeID = strings.TrimSpace(string(payload))
	}
	if id == activeID {
		return fmt.Errorf("cannot remove active profile")
	}
	file, err := s.read()
	if err != nil {
		return err
	}
	next := file.Profiles[:0]
	removed := false
	for _, profile := range file.Profiles {
		if profile.ID == id {
			removed = true
			continue
		}
		next = append(next, profile)
	}
	if !removed {
		return fmt.Errorf("profile %q not found", id)
	}
	file.Profiles = next
	return s.write(file)
}

func (s Store) SetCamouflageOverride(id string, site string) error {
	file, err := s.read()
	if err != nil {
		return err
	}
	for i := range file.Profiles {
		if file.Profiles[i].ID == id {
			file.Profiles[i].CamouflageSiteOverride = strings.TrimSpace(site)
			return s.write(file)
		}
	}
	return fmt.Errorf("profile %q not found", id)
}

func (p Profile) Bundle() (bundle.Bundle, error) {
	return bundle.Decode(p.Link)
}

func (p Profile) CamouflageSite() string {
	if p.CamouflageSiteOverride != "" {
		return p.CamouflageSiteOverride
	}
	b, err := p.Bundle()
	if err != nil {
		return ""
	}
	return b.CamouflageSite()
}

func (s Store) read() (File, error) {
	payload, err := os.ReadFile(s.path())
	if os.IsNotExist(err) {
		return File{Version: 1}, nil
	}
	if err != nil {
		return File{}, err
	}
	var file File
	if err := json.Unmarshal(payload, &file); err != nil {
		return File{}, err
	}
	if file.Version == 0 {
		file.Version = 1
	}
	return file, nil
}

func (s Store) write(file File) error {
	if err := os.MkdirAll(s.stateDir, 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), append(payload, '\n'), 0600)
}

func (s Store) path() string {
	return filepath.Join(s.stateDir, "profiles.json")
}

func (s Store) activePath() string {
	return filepath.Join(s.stateDir, "active_profile_id.txt")
}
