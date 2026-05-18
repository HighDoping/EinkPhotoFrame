package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config contains runtime settings loaded from environment variables.
type Config struct {
	AdminKey        string
	ImageDir        string
	ImageDirRefresh int
	CacheDir        string
	CertFile        string
	KeyFile         string
}

func LoadFromEnv() Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	cfg := Config{}

	cfg.AdminKey = os.Getenv("ADMIN_KEY")
	if cfg.AdminKey == "" {
		log.Println("Warning: ADMIN_TOKEN not set in .env, using default (not secure for production)")
		cfg.AdminKey = "default_admin_token"
	}
	log.Println("Using admin token:", cfg.AdminKey)

	cfg.ImageDir = os.Getenv("IMAGE_DIR")
	if cfg.ImageDir == "" {
		log.Println("Warning: IMAGE_DIR not set in .env, using default")
		cfg.ImageDir = "./images"
	}
	log.Println("Using image directory:", cfg.ImageDir)

	cfg.ImageDirRefresh, err = strconv.Atoi(os.Getenv("IMAGE_DIR_REFRESH"))
	if err != nil {
		log.Println("Warning: IMAGE_DIR_REFRESH not set in .env, using default")
		cfg.ImageDirRefresh = 86400
	}

	cfg.CacheDir = os.Getenv("CACHE_DIR")
	if cfg.CacheDir == "" {
		log.Println("Warning: CACHE_DIR not set in .env, using default")
		cfg.CacheDir, _ = os.UserCacheDir()
	}
	cfg.CertFile = os.Getenv("CERT_FILE")
	cfg.KeyFile = os.Getenv("KEY_FILE")

	return cfg
}
