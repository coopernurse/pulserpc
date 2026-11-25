## Design

Use the existing fast path for bare integer tokens. Check decimal and exponent tokens before converting them to float64. Preserve tokens that cannot safely become integers so IDL integer validation can reject them. Canonicalize exact wide integers as json.Number and leave ordinary safe integers as float64.

Bound exponent processing by the input length and construct at most 19 significant decimal digits before signed parsing. Numeric conversion recurses through optional pointer targets and allocates only after the element conversion succeeds. Float contracts keep their existing float64 interpretation.
