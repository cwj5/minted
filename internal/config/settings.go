package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Settings represents all application configuration
type Settings struct {
	Variables   map[string]string      `json:"variables"`
	Tiers       []Tier                 `json:"tiers"`
	Theme       string                 `json:"theme"`
	Preferences map[string]interface{} `json:"preferences"`
}

// Tier represents a spending tier with assigned categories
type Tier struct {
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Color      string   `json:"color"`
}

// DefaultSettings returns settings with sensible defaults
func DefaultSettings() *Settings {
	return &Settings{
		Variables: map[string]string{
			"HLEDGER_FILE": "$HOME/.local/share/hledger/journal.journal",
			"PORT":         "9999",
		},
		Tiers: []Tier{
			{
				Name:       "Essential",
				Categories: []string{"Groceries", "Utilities", "Insurance", "Transport"},
				Color:      "#27ae60",
			},
			{
				Name:       "Discretionary",
				Categories: []string{"Entertainment", "Dining", "Shopping", "Hobbies"},
				Color:      "#e74c3c",
			},
			{
				Name:       "Fixed",
				Categories: []string{"Rent", "Subscriptions", "Phone"},
				Color:      "#3498db",
			},
		},
		Theme: "light",
		Preferences: map[string]interface{}{
			"transactionLimit": 0,
			"defaultDateRange": "6months",
		},
	}
}

// LoadSettings loads settings from ${MINTED_DIR}/settings.json
func LoadSettings() (*Settings, error) {
	mintedDir := os.Getenv("MINTED_DIR")
	if mintedDir == "" {
		return nil, fmt.Errorf("MINTED_DIR environment variable not set")
	}

	settingsPath := filepath.Join(mintedDir, "settings.json")

	// Check if file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		// Create default settings if file doesn't exist
		settings := DefaultSettings()
		if err := SaveSettings(settings); err != nil {
			return nil, fmt.Errorf("failed to create default settings: %w", err)
		}
		return settings, nil
	}

	// Read the file
	data, err := ioutil.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	// Parse JSON
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings file: %w", err)
	}

	return &settings, nil
}

// SaveSettings saves settings to ${MINTED_DIR}/settings.json
func SaveSettings(settings *Settings) error {
	mintedDir := os.Getenv("MINTED_DIR")
	if mintedDir == "" {
		return fmt.Errorf("MINTED_DIR environment variable not set")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(mintedDir, 0755); err != nil {
		return fmt.Errorf("failed to create MINTED_DIR: %w", err)
	}

	settingsPath := filepath.Join(mintedDir, "settings.json")

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// GetVariableValue retrieves an environment variable value from settings
func (s *Settings) GetVariableValue(key string) string {
	if val, exists := s.Variables[key]; exists {
		return os.ExpandEnv(val)
	}
	return ""
}

// GetTierForCategory finds which tier a category belongs to
func (s *Settings) GetTierForCategory(category string) *Tier {
	for i := range s.Tiers {
		for _, cat := range s.Tiers[i].Categories {
			if cat == category {
				return &s.Tiers[i]
			}
		}
	}
	return nil
}

// AddCategory adds a category to a tier
func (s *Settings) AddCategory(tierName, category string) error {
	for i := range s.Tiers {
		if s.Tiers[i].Name == tierName {
			// Check if already exists
			for _, cat := range s.Tiers[i].Categories {
				if cat == category {
					return fmt.Errorf("category already exists in tier")
				}
			}
			s.Tiers[i].Categories = append(s.Tiers[i].Categories, category)
			return nil
		}
	}
	return fmt.Errorf("tier not found")
}

// RemoveCategory removes a category from a tier
func (s *Settings) RemoveCategory(tierName, category string) error {
	for i := range s.Tiers {
		if s.Tiers[i].Name == tierName {
			for j, cat := range s.Tiers[i].Categories {
				if cat == category {
					s.Tiers[i].Categories = append(s.Tiers[i].Categories[:j], s.Tiers[i].Categories[j+1:]...)
					return nil
				}
			}
		}
	}
	return fmt.Errorf("category not found in tier")
}

// CreateTier creates a new spending tier
func (s *Settings) CreateTier(name, color string) error {
	for _, tier := range s.Tiers {
		if tier.Name == name {
			return fmt.Errorf("tier already exists")
		}
	}
	s.Tiers = append(s.Tiers, Tier{
		Name:       name,
		Categories: []string{},
		Color:      color,
	})
	return nil
}

// DeleteTier deletes a spending tier
func (s *Settings) DeleteTier(name string) error {
	for i, tier := range s.Tiers {
		if tier.Name == name {
			s.Tiers = append(s.Tiers[:i], s.Tiers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("tier not found")
}
