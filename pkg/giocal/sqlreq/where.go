package sqlreq

import (
	"CLI-Geographic-Calculation/pkg/giocal/giocaltype"
	"CLI-Geographic-Calculation/pkg/giocal/linefilter"
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

/*
*

	spl_parser_test.go:57: bool_expr:{boolop:AND_EXPR args:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"company"}} location:25}} rexpr:{a_const:{sval:{sval:"東日本旅客鉄道"} location:35}} location:33}} args:{a_expr:{kind:AEXPR_IN name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"line"}} location:63}} rexpr:{list:{items:{a_const:{sval:{sval:"山手線"} location:72}} items:{a_const:{sval:{sval:"中央線"} location:85}}}} location:68}} location:59}
*/
func ParseWhereClause[T giocaltype.GiotypeFeatureConstraint](filterFunc linefilter.FilterByProperties[T], drsf *[]T, whereClause *pg_query.Node, required []int) []int {
	// 要素が2つ以上ある場合
	/**
	var required1 []int
	if isAexpr(left) {
		required1 = ParseWhereClause(drs, left, required)
	} else {
		required1 = []int{}
	}

	var required2 []int
	if isAexpr(right) {
		required2 = ParseWhereClause(drs, right, required)
	} else {
		required2 = []int{}
	}
		これを参考に
	*/

	print("* * * [PWC/処理開始] WhereClause:", whereClause.String(), "\n")

	if !isExper(whereClause) {
		// A_Exprでない場合は処理しない
		print("[PWC/終了] WhereClauseは式ではありません:", whereClause.String(), "\n")
		return required
	}

	var requireds [][]int = [][]int{}

	//子要素が2つの時
	if whereClause.GetAExpr() != nil {
		lexpr := whereClause.GetAExpr().GetLexpr()
		rexpr := whereClause.GetAExpr().GetRexpr()

		column, values, ok := ParseWhereExprIn(lexpr, rexpr)
		print("[PWC/A_Expr] Column:", column, " Values:", printStringArray(values), "\n")
		if ok {
			req := filterFunc(drsf, column, values)
			requireds = append(requireds, req)
			print("[PWC/A_Expr] 要素追加:", printIntArray(req), "\n")
		}
	} else {
		// 子要素数
		for _, arg := range whereClause.GetBoolExpr().GetArgs() {
			if isExper(arg) {
				print("[PWC/再帰] 子要素処理開始:", arg.String(), "\n")
				req := ParseWhereClause(filterFunc, drsf, arg, required)
				requireds = append(requireds, req)
			}
		}
	}

	print("[PWC/結合前] 要素数:", len(requireds), " 要素一覧:")
	for _, r := range requireds {
		print(" ", printIntArray(r))
	}
	print("\n")

	if len(requireds) == 0 {
		return required
	}

	print("[PWC/結合開始] WhereClause:", whereClause.String(), "\n")
	// 結合
	result := requireds[0]
	for i := 1; i < len(requireds); i++ {
		if whereClause.GetBoolExpr().GetBoolop() == pg_query.BoolExprType_AND_EXPR {
			result = WhereAnd(result, requireds[i])
			print("[PWC/AND] 結合結果:", printIntArray(result), "\n")
		} else if whereClause.GetBoolExpr().GetBoolop() == pg_query.BoolExprType_OR_EXPR {
			result = WhereOr(result, requireds[i])
			print("[PWC/OR] 結合結果:", printIntArray(result), "\n")
		}
	}
	print("[PWC/処理終了] WhereClause:", whereClause.String(), " 結果:", printIntArray(result), "\n")
	return result
}

// string配列を出力するhelper
func printStringArray(arr []string) string {
	return fmt.Sprintf("%v", arr)
}
func printIntArray(arr []int) string {
	return fmt.Sprintf("%v", arr)
}

/**
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
*/

/**
bool_expr  (boolop=OR_EXPR)
├─ args[0]: bool_expr (boolop=AND_EXPR)
│  ├─ args[0]: a_expr (kind=AEXPR_OP, op="=")
│  │  ├─ lexpr: column_ref
│  │  │  └─ fields[0]: string "company"  (location=26)
│  │  └─ rexpr: a_const (string)
│  │     └─ sval "東日本旅客鉄道"  (location=36)
│  └─ args[1]: a_expr (kind=AEXPR_IN)
│     ├─ lexpr: column_ref
│     │  └─ fields[0]: string "line"  (location=64)
│     └─ rexpr: list  (location=69)
│        ├─ items[0]: a_const (string) sval "山手線"  (location=73)
│        └─ items[1]: a_const (string) sval "中央線"  (location=86)
└─ args[1]: a_expr (kind=AEXPR_OP, op="=")  (location=100)
   ├─ lexpr: column_ref
   │  └─ fields[0]: string "year"  (location=103)
   └─ rexpr: a_const (int)
      └─ ival 2023  (location=110)

*/

func isExper(node *pg_query.Node) bool {
	return isAexpr(node) || isBoolExpr(node)
}

func isBoolExpr(node *pg_query.Node) bool {
	return node.GetBoolExpr() != nil
}

func isAexpr(node *pg_query.Node) bool {
	return node.GetAExpr() != nil
}

// tree
type ParserNode func(drs *giocaltype.DatasetResource, required1 []int, required2 []int) []int

// 必要な行数を返す
func WhereAnd(required1 []int, required2 []int) []int {

	// required1とrequired2の共通部分を返す
	result := make([]int, 0)
	m := make(map[int]bool)

	for _, v := range required1 {
		m[v] = true
	}

	for _, v := range required2 {
		if m[v] {
			result = append(result, v)
		}
	}

	return result
}

// 必要な行数を返す
func WhereOr(required1 []int, required2 []int) []int {
	// required1とrequired2の和集合を返す
	result := make([]int, 0)
	m := make(map[int]bool)

	for _, v := range required1 {
		m[v] = true
	}

	for _, v := range required2 {
		m[v] = true
	}

	for k := range m {
		result = append(result, k)
	}

	return result
}

/**
  │  │  ├─ left : ColumnRef("line") (location: 63)
  │  │  ├─ right: List
  │  │  │  ├─ ConstString("山手線") (location: 72)
  │  │  └  └─ ConstString("中央線") (location: 85)
*/

/**
BoolExprType_BOOL_EXPR_TYPE_UNDEFINED BoolExprType = 0
BoolExprType_AND_EXPR                 BoolExprType = 1
BoolExprType_OR_EXPR                  BoolExprType = 2
BoolExprType_NOT_EXPR                 BoolExprType = 3
*/

func ParseWhereExprIn(
	left *pg_query.Node,
	right *pg_query.Node,
) (string, []string, bool) {

	if left == nil || right == nil {
		return "", nil, false
	}

	// 左辺: ColumnRef
	colRef := left.GetColumnRef()
	if colRef == nil {
		return "", nil, false
	}

	// column 名取得（単一カラム想定）
	if len(colRef.Fields) != 1 {
		return "", nil, false
	}
	column := colRef.Fields[0].GetString_().GetSval()

	// 右辺が単一であれば、INではなく=の可能性もある
	if c := right.GetAConst(); c != nil {
		if sval := c.GetSval(); sval != nil {
			return column, []string{sval.Sval}, true
		}
		return "", nil, false
	}

	// 右辺: IN (...) の List
	list := right.GetList()
	if list == nil {
		return "", nil, false
	}

	values := make([]string, 0, len(list.Items))

	for _, item := range list.Items {
		c := item.GetAConst()
		if c == nil {
			continue
		}

		if sval := c.GetSval(); sval != nil {
			values = append(values, sval.Sval)
		}
	}

	if len(values) == 0 {
		return "", nil, false
	}

	return column, values, true
}
