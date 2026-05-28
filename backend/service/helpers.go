package service

import "gorm.io/gorm/clause"

func gormExpr(expr string, args ...interface{}) clause.Expr {
	return clause.Expr{SQL: expr, Vars: args}
}
