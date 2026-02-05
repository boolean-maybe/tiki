---
title: Inconsistent formatting in monthly reports
type: bug
status: backlog
priority: 3
tags:
  - quality
  - reports
---

# Report Formatting Issues

This is just a sample tiki - feel free to remove!

## Problem Description

> Multiple teams have reported inconsistent formatting in monthly status reports, making them difficult to compare and analyze trends over time.

## Examples of Issues

**Header inconsistency:**
```
Report A: "Monthly Status - January 2024"
Report B: "Jan 2024 Status Report"
Report C: "Status Update (2024-01)"
```

**Date format variations:**
```
- MM/DD/YYYY
- DD-MM-YYYY
- YYYY-MM-DD
```

## Impact

1. Reports are harder to review
2. Automated processing fails
3. Historical comparison is challenging
4. Professional appearance suffers

## Proposed Solution

> Create a standardized report template with:
> - Fixed header format
> - Consistent date formatting (ISO 8601)
> - Standard section ordering
> - Style guide for common elements

## Related Documentation

- Style Guide: `docs/style-guide.md`
- Report Archive: `reports/archive/`
