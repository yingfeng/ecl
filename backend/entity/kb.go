package entity

type Knowledgebase struct {
	ID                     string     `gorm:"column:id;primaryKey;size:32" json:"id"`
	Avatar                 *string    `gorm:"column:avatar;type:longtext" json:"avatar,omitempty"`
	TenantID               string     `gorm:"column:tenant_id;size:32;not null;index" json:"tenant_id"`
	Name                   string     `gorm:"column:name;size:128;not null;index" json:"name"`
	Language               *string    `gorm:"column:language;size:32;index" json:"language,omitempty"`
	Description            *string    `gorm:"column:description;type:longtext" json:"description,omitempty"`
	EmbdID                 string     `gorm:"column:embd_id;size:128;not null;index" json:"embd_id"`
	TenantEmbdID           *int64     `gorm:"column:tenant_embd_id;index" json:"tenant_embd_id,omitempty"`
	Permission             string     `gorm:"column:permission;size:16;not null;default:me;index" json:"permission"`
	CreatedBy              string     `gorm:"column:created_by;size:32;not null;index" json:"created_by"`
	DocNum                 int64      `gorm:"column:doc_num;default:0;index" json:"doc_num"`
	TokenNum               int64      `gorm:"column:token_num;default:0;index" json:"token_num"`
	ChunkNum               int64      `gorm:"column:chunk_num;default:0;index" json:"chunk_num"`
	SimilarityThreshold    float64    `gorm:"column:similarity_threshold;default:0.2" json:"similarity_threshold"`
	VectorSimilarityWeight float64    `gorm:"column:vector_similarity_weight;default:0.3" json:"vector_similarity_weight"`
	ParserID               string     `gorm:"column:parser_id;size:32;not null;default:naive;index" json:"parser_id"`
	PipelineID             *string    `gorm:"column:pipeline_id;size:32;index" json:"pipeline_id,omitempty"`
	ParserConfig           JSONMap    `gorm:"column:parser_config;type:json" json:"parser_config"`
	Pagerank               int64      `gorm:"column:pagerank;default:0" json:"pagerank"`
	Status                 *string    `gorm:"column:status;size:1;index" json:"status,omitempty"`
	BaseModel
}

func (Knowledgebase) TableName() string { return "knowledgebase" }
