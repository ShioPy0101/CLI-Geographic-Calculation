package sqlreq

//github.com/pganalyze/pg_query_go/v6を使う
import (
	"CLI-Geographic-Calculation/pkg/giocal"
	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/graphstructure"
	"CLI-Geographic-Calculation/pkg/giocal/linefilter"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func ParseSQLQuery(query string) *pg_query.ParseResult {
	parsed, err := ParseSQLQueryE(query)
	if err != nil {
		panic(err)
	}
	return parsed
}

func ParseSQLQueryE(query string) (*pg_query.ParseResult, error) {
	return pg_query.Parse(query)
}

/**
{"version":170004,"stmts":[{"stmt":{"SelectStmt":{"targetList":[{"ResTarget":{"val":{"ColumnRef":{"fields":[{"A_Star":{}}],"location":7}},"location":7}}],"fromClause":[{"RangeVar":{"relname":"rail","inh":true,"relpersistence":"p","location":14}}],"whereClause":{"A_Expr":{"kind":"AEXPR_OP","name":[{"String":{"sval":"="}}],"lexpr":{"ColumnRef":{"fields":[{"String":{"sval":"year"}}],"location":25}},"rexpr":{"A_Const":{"ival":{"ival":2023},"location":32}},"location":30}},"limitOption":"LIMIT_OPTION_DEFAULT","op":"SETOP_NONE"}}}]}
*/
/**
	"SELECT * FROM rail WHERE company = '東日本旅客鉄道' AND line IN ('山手線', '中央線')"
    spl_parser_test.go:33: version:170004 stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}} location:7}} location:7}} from_clause:{range_var:{relname:"rail" inh:true relpersistence:"p" location:14}} where_clause:{bool_expr:{boolop:AND_EXPR args:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"company"}} location:25}} rexpr:{a_const:{sval:{sval:"東日本旅客鉄道"} location:35}} location:33}} args:{a_expr:{kind:AEXPR_IN name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"line"}} location:63}} rexpr:{list:{items:{a_const:{sval:{sval:"山手線"} location:72}} items:{a_const:{sval:{sval:"中央線"} location:85}}}} location:68}} location:59}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE}}}
*/
/**
ParseResult
├─ version: 170004
└─ stmts[0]
   └─ stmt: SelectStmt
      ├─ target_list
      │  └─ ResTarget
      │     └─ val: ColumnRef
      │        └─ fields: [*]   (A_Star)
      │        └─ location: 7
      ├─ from_clause
      │  └─ RangeVar
      │     ├─ relname: "rail"
      │     ├─ inh: true
      │     ├─ relpersistence: "p"
      │     └─ location: 14
      ├─ where_clause: BoolExpr
      │  ├─ boolop: AND_EXPR
      │  ├─ args[0]: A_Expr
      │  │  ├─ kind: AEXPR_OP           (通常の二項演算)
      │  │  ├─ op: "="
      │  │  ├─ left : ColumnRef("company")  (location: 25)
      │  │  ├─ right: ConstString("東日本旅客鉄道") (location: 35)
      │  │  └─ location: 33
      │  ├─ args[1]: A_Expr
      │  │  ├─ kind: AEXPR_IN           (IN 条件)
      │  │  ├─ left : ColumnRef("line") (location: 63)
      │  │  ├─ right: List
      │  │  │  ├─ ConstString("山手線") (location: 72)
      │  │  └  └─ ConstString("中央線") (location: 85)
      │  │  └─ location: 68
      │  └─ location: 59
      ├─ limit_option: LIMIT_OPTION_DEFAULT
      └─ op: SETOP_NONE
*/

func GetFirstStmt(parsed *pg_query.ParseResult) *pg_query.Node {
	return parsed.Stmts[0].Stmt
}

func GetFromClause(stmt *pg_query.Node) *pg_query.Node {
	selectStmt := stmt.GetSelectStmt()
	return selectStmt.FromClause[0]
}

func GetWhereClause(stmt *pg_query.Node) *pg_query.Node {
	selectStmt := stmt.GetSelectStmt()
	return selectStmt.WhereClause
}

func GetGroupByClauses(stmt *pg_query.Node) []*pg_query.Node {
	selectStmt := stmt.GetSelectStmt()
	return selectStmt.GroupClause
}

func SQLToGraph(filterFunc linefilter.FilterByProperties[giocaltype.GiotypeRailroadSection], parsed *pg_query.ParseResult, drs *giocaltype.DatasetResource) *graphstructure.Graph {
	firstStmt := GetFirstStmt(parsed)
	whereClause := GetWhereClause(firstStmt)
	railroadSectionRequired := ParseWhereClause(filterFunc, &drs.Rail.Features, whereClause, []int{})
	stationRequired := ParseWhereClause(linefilter.FilterStationByProperties, &drs.Station.Features, whereClause, []int{})
	graph := giocal.ConvertGiotypeRailwayToGraphByRequired(drs.Station, drs.Rail, stationRequired, railroadSectionRequired)
	return graph
}
