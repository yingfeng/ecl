package entity

import (
	"strings"

	"github.com/google/uuid"
)

// NewID generates a new UUID without dashes (32 chars) matching the Python Peewee schema.
func NewID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

const (
	DatasetNameLimit = 128
)

type Status string

const (
	StatusValid   Status = "1"
	StatusInvalid Status = "0"
)

type TenantPermission string

const (
	TenantPermissionMe   TenantPermission = "me"
	TenantPermissionTeam TenantPermission = "team"
)

type ParserType string

const (
	ParserTypeNaive   ParserType = "naive"
	ParserTypeBook    ParserType = "book"
	ParserTypeLaws    ParserType = "laws"
	ParserTypeQA      ParserType = "qa"
	ParserTypeTable   ParserType = "table"
	ParserTypePicture ParserType = "picture"
	ParserTypeOne     ParserType = "one"
	ParserTypeAudio   ParserType = "audio"
	ParserTypeEmail   ParserType = "email"
	ParserTypeKG      ParserType = "knowledge_graph"
	ParserTypeTag     ParserType = "tag"
)

type FileSource string

const (
	FileSourceLocal         FileSource = ""
	FileSourceKnowledgebase FileSource = "knowledgebase"
)

const (
	KnowledgebaseFolderName = ".knowledgebase"
	SkillsFolderName        = "skills"
)
