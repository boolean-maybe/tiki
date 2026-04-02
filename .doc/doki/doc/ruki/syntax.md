# Syntax

## Table of contents

- [Overview](#overview)
- [Lexical structure](#lexical-structure)
- [Top-level grammar](#top-level-grammar)
- [Condition grammar](#condition-grammar)
- [Expression grammar](#expression-grammar)
- [Operator binding summary](#operator-binding-summary)
- [Syntax notes](#syntax-notes)

## Overview

This page describes Ruki syntax. It starts with tokens and then shows the grammar for statements, triggers, conditions, and expressions.

## Lexical structure

Ruki uses these token classes:

- comments: `--` to end of line
- whitespace: ignored between tokens
- durations: `\d+(sec|min|hour|day|week|month|year)s?`
- dates: `YYYY-MM-DD`
- integers: decimal digits only
- strings: double-quoted strings with backslash escapes
- comparison operators: `=`, `!=`, `<`, `>`, `<=`, `>=`
- binary operators: `+`, `-`
- punctuation: `.`, `(`, `)`, `[`, `]`, `,`
- identifiers: `[a-zA-Z_][a-zA-Z0-9_]*`

Examples:

```sql
-- line comment
2026-03-25
2day
"hello"
dependsOn
new.status
```

## Top-level grammar

The following EBNF-style summary shows the grammar:

```text
statement        = selectStmt | createStmt | updateStmt | deleteStmt ;

selectStmt       = "select" [ "where" condition ] ;
createStmt       = "create" assignment { assignment } ;
updateStmt       = "update" "where" condition "set" assignment { assignment } ;
deleteStmt       = "delete" "where" condition ;

assignment       = identifier "=" expr ;

trigger          = timing event [ "where" condition ] ( action | deny ) ;
timing           = "before" | "after" ;
event            = "create" | "update" | "delete" ;

action           = runAction | createStmt | updateStmt | deleteStmt ;
runAction        = "run" "(" expr ")" ;
deny             = "deny" string ;
```

Notes:

- `select` is a valid top-level statement, but it is not valid as a trigger action.
- `create` requires at least one assignment.
- `update` requires both `where` and `set`.
- `delete` requires `where`.

## Condition grammar

Condition precedence follows this order:

```text
condition        = orCond ;
orCond           = andCond { "or" andCond } ;
andCond          = notCond { "and" notCond } ;
notCond          = "not" notCond | primaryCond ;
primaryCond      = "(" condition ")" | exprCond ;

exprCond         = expr
                   [ compareTail
                   | isEmptyTail
                   | isNotEmptyTail
                   | notInTail
                   | inTail
                   | anyTail
                   | allTail ] ;

compareTail      = compareOp expr ;
isEmptyTail      = "is" "empty" ;
isNotEmptyTail   = "is" "not" "empty" ;
inTail           = "in" expr ;
notInTail        = "not" "in" expr ;
anyTail          = "any" primaryCond ;
allTail          = "all" primaryCond ;
```

Examples:

```sql
select where status = "done"
select where assignee is empty
select where status not in ["done", "cancelled"]
select where dependsOn any status != "done"
select where not (status = "done" or priority = 1)
```

## Expression grammar

Expressions support literals, field references, qualifiers, function calls, list literals, parenthesized expressions, subqueries, and left-associative `+` or `-` chains:

```text
expr             = unaryExpr { ("+" | "-") unaryExpr } ;

unaryExpr        = funcCall
                 | subQuery
                 | qualifiedRef
                 | listLiteral
                 | string
                 | date
                 | duration
                 | int
                 | emptyLiteral
                 | fieldRef
                 | "(" expr ")" ;

funcCall         = identifier "(" [ expr { "," expr } ] ")" ;
subQuery         = "select" [ "where" condition ] ;
qualifiedRef     = ( "old" | "new" ) "." identifier ;
listLiteral      = "[" [ expr { "," expr } ] "]" ;
emptyLiteral     = "empty" ;
fieldRef         = identifier ;
```

Examples:

```sql
title
old.status
["bug", "frontend"]
next_date(recurrence)
count(select where status = "done")
2026-03-25 + 2day
tags + ["needs-triage"]
```

## Operator binding summary

Condition operators:

- highest: a condition in parentheses, or a condition built from a single expression
- then: `not`
- then: `and`
- lowest: `or`

Expression operators:

- only one binary precedence level exists for expressions
- `+` and `-` associate left to right

That means:

```sql
select where priority = 1 or priority = 2 and status = "done"
```

parses as:

```text
priority = 1 or (priority = 2 and status = "done")
```

## Syntax notes

- `any` and `all` apply to the condition that comes right after them. If you want to combine that condition with `and` or `or`, use parentheses.
- `select` used inside expressions is only valid as a `count(...)` argument. Bare subqueries are rejected during validation.
- The grammar accepts `run(<expr>)`, but only as the top-level action of an `after` trigger.
- `old.` and `new.` are only allowed in some trigger conditions. See [Semantics](semantics.md) and [Validation And Errors](validation-and-errors.md).
