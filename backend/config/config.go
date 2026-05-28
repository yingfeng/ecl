package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerHost string
	ServerPort string
	MySQLDSN   string
	MinIOHost  string
	MinIOUser  string
	MinIOPass  string
	MinIOSecure bool
	MinIOBucket string
}

type yamlConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port string `yaml:"port"`
	} `yaml:"server"`
	MySQL struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"mysql"`
	MinIO struct {
		Host     string `yaml:"host"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Secure   bool   `yaml:"secure"`
	} `yaml:"minio"`
}

func Load() *Config {
	yc := &yamlConfig{}
	yc.Server.Host = "0.0.0.0"
	yc.Server.Port = "8080"
	yc.MySQL.Host = "127.0.0.1"
	yc.MySQL.Port = 3306
	yc.MySQL.User = "rag_flow"
	yc.MySQL.Password = "infini_rag_flow"
	yc.MySQL.Database = "rag_flow"
	yc.MinIO.Host = "127.0.0.1:9000"
	yc.MinIO.User = "rag_flow"
	yc.MinIO.Password = "infini_rag_flow"

	// 尝试从多个位置加载 YAML 配置文件
	tryLoadYAML(yc, "config.yaml")
	tryLoadYAML(yc, filepath.Join("config", "config.yaml"))
	tryLoadYAML(yc, filepath.Join("backend", "config", "config.yaml"))

	// 环境变量覆盖（最高优先级）
	if v := os.Getenv("SERVER_HOST"); v != "" {
		yc.Server.Host = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		yc.Server.Port = v
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		yc.MySQL.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &yc.MySQL.Port)
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		yc.MySQL.User = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		yc.MySQL.Password = v
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		yc.MySQL.Database = v
	}
	if v := os.Getenv("MINIO_HOST"); v != "" {
		yc.MinIO.Host = v
	}
	if v := os.Getenv("MINIO_USER"); v != "" {
		yc.MinIO.User = v
	}
	if v := os.Getenv("MINIO_PASSWORD"); v != "" {
		yc.MinIO.Password = v
	}
	if v := os.Getenv("MINIO_SECURE"); v == "true" {
		yc.MinIO.Secure = true
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		yc.MySQL.User, yc.MySQL.Password,
		yc.MySQL.Host, yc.MySQL.Port,
		yc.MySQL.Database)

	return &Config{
		ServerHost: yc.Server.Host,
		ServerPort: yc.Server.Port,
		MySQLDSN:   getEnv("MYSQL_DSN", dsn),
		MinIOHost:  yc.MinIO.Host,
		MinIOUser:  yc.MinIO.User,
		MinIOPass:  yc.MinIO.Password,
		MinIOSecure: yc.MinIO.Secure,
		MinIOBucket: "llmwiki",
	}
}

func tryLoadYAML(yc *yamlConfig, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := yaml.Unmarshal(data, yc); err == nil {
		fmt.Printf("Loaded config from %s\n", path)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
