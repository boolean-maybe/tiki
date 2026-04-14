package ruki

// grammar.go — unexported participle grammar structs.
// these encode operator precedence via grammar layering.
// consumers never see these; lower.go converts them to clean AST types.

// --- top-level statement grammar ---

type statementGrammar struct {
	Select *selectGrammar `parser:"  @@"`
	Create *createGrammar `parser:"| @@"`
	Update *updateGrammar `parser:"| @@"`
	Delete *deleteGrammar `parser:"| @@"`
}

type fieldNamesGrammar struct {
	First string   `parser:"@Ident"`
	Rest  []string `parser:"( ',' @Ident )*"`
}

type selectGrammar struct {
	Star    *string            `parser:"'select' ( @Star"`
	Fields  *fieldNamesGrammar `parser:"           | @@ )?"`
	Where   *orCond            `parser:"( 'where' @@ )?"`
	OrderBy *orderByGrammar    `parser:"@@?"`
	Pipe    *runGrammar        `parser:"( Pipe @@ )?"`
}

// --- order by grammar ---

type orderByGrammar struct {
	First orderByField   `parser:"'order' 'by' @@"`
	Rest  []orderByField `parser:"( ',' @@ )*"`
}

type orderByField struct {
	Field     string  `parser:"@Ident"`
	Direction *string `parser:"@( 'asc' | 'desc' )?"`
}

type createGrammar struct {
	Assignments []assignmentGrammar `parser:"'create' @@+"`
}

type updateGrammar struct {
	Where orCond              `parser:"'update' 'where' @@"`
	Set   []assignmentGrammar `parser:"'set' @@+"`
}

type deleteGrammar struct {
	Where orCond `parser:"'delete' 'where' @@"`
}

type assignmentGrammar struct {
	Field string      `parser:"@Ident '='"`
	Value exprGrammar `parser:"@@"`
}

// --- trigger grammar ---

type triggerGrammar struct {
	Timing string         `parser:"@( 'before' | 'after' )"`
	Event  string         `parser:"@( 'create' | 'update' | 'delete' )"`
	Where  *orCond        `parser:"( 'where' @@ )?"`
	Action *actionGrammar `parser:"( @@"`
	Deny   *denyGrammar   `parser:"| @@ )?"`
}

type actionGrammar struct {
	Run    *runGrammar    `parser:"  @@"`
	Create *createGrammar `parser:"| @@"`
	Update *updateGrammar `parser:"| @@"`
	Delete *deleteGrammar `parser:"| @@"`
}

type runGrammar struct {
	Command exprGrammar `parser:"'run' '(' @@ ')'"`
}

type denyGrammar struct {
	Message string `parser:"'deny' @String"`
}

// --- time trigger grammar ---

type timeTriggerGrammar struct {
	Interval string         `parser:"'every' @Duration"`
	Create   *createGrammar `parser:"(  @@"`
	Update   *updateGrammar `parser:"| @@"`
	Delete   *deleteGrammar `parser:"| @@ )"`
}

// --- rule grammar (union of event trigger and time trigger) ---

type ruleGrammar struct {
	TimeTrigger *timeTriggerGrammar `parser:"  @@"`
	Trigger     *triggerGrammar     `parser:"| @@"`
}

// --- condition grammar (precedence layers) ---

// orCond is the lowest-precedence condition layer.
type orCond struct {
	Left  andCond   `parser:"@@"`
	Right []andCond `parser:"( 'or' @@ )*"`
}

type andCond struct {
	Left  notCond   `parser:"@@"`
	Right []notCond `parser:"( 'and' @@ )*"`
}

type notCond struct {
	Not     *notCond     `parser:"  'not' @@"`
	Primary *primaryCond `parser:"| @@"`
}

type primaryCond struct {
	Paren *orCond   `parser:"  '(' @@ ')'"`
	Expr  *exprCond `parser:"| @@"`
}

// exprCond parses an expression followed by a condition operator.
type exprCond struct {
	Left       exprGrammar     `parser:"@@"`
	Compare    *compareTail    `parser:"( @@"`
	IsEmpty    *isEmptyTail    `parser:"| @@"`
	IsNotEmpty *isNotEmptyTail `parser:"| @@"`
	NotIn      *notInTail      `parser:"| @@"`
	In         *inTail         `parser:"| @@"`
	Any        *quantifierTail `parser:"| @@"`
	All        *allQuantTail   `parser:"| @@ )?"`
}

type compareTail struct {
	Op    string      `parser:"@CompareOp"`
	Right exprGrammar `parser:"@@"`
}

type isEmptyTail struct {
	Is string `parser:"@'is' 'empty'"`
}

type isNotEmptyTail struct {
	Is string `parser:"@'is' 'not' 'empty'"`
}

type inTail struct {
	Collection exprGrammar `parser:"'in' @@"`
}

type notInTail struct {
	Collection exprGrammar `parser:"'not' 'in' @@"`
}

type quantifierTail struct {
	Condition primaryCond `parser:"'any' @@"`
}

type allQuantTail struct {
	Condition primaryCond `parser:"'all' @@"`
}

// --- expression grammar ---

type exprGrammar struct {
	Left unaryExpr     `parser:"@@"`
	Tail []exprBinTail `parser:"@@*"`
}

type exprBinTail struct {
	Op    string    `parser:"@( Plus | Minus )"`
	Right unaryExpr `parser:"@@"`
}

type unaryExpr struct {
	FuncCall *funcCallExpr `parser:"  @@"`
	SubQuery *subQueryExpr `parser:"| @@"`
	QualRef  *qualRefExpr  `parser:"| @@"`
	ListLit  *listLitExpr  `parser:"| @@"`
	StrLit   *string       `parser:"| @String"`
	DateLit  *string       `parser:"| @Date"`
	DurLit   *string       `parser:"| @Duration"`
	IntLit   *int          `parser:"| @Int"`
	Empty    *emptyExpr    `parser:"| @@"`
	FieldRef *string       `parser:"| @Ident"`
	Paren    *exprGrammar  `parser:"| '(' @@ ')'"`
}

type funcCallExpr struct {
	Name string        `parser:"@Ident '('"`
	Args []exprGrammar `parser:"( @@ ( ',' @@ )* )? ')'"`
}

type subQueryExpr struct {
	Where   *orCond         `parser:"'select' ( 'where' @@ )?"`
	OrderBy *orderByGrammar `parser:"@@?"`
}

type qualRefExpr struct {
	Qualifier string `parser:"@( 'old' | 'new' ) '.'"`
	Name      string `parser:"@Ident"`
}

type listLitExpr struct {
	Elements []exprGrammar `parser:"'[' ( @@ ( ',' @@ )* )? ']'"`
}

type emptyExpr struct {
	Keyword string `parser:"@'empty'"`
}
