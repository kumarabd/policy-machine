package authz

default allow := false
default reason := "no matching permission"
default obligations := []

# --- Condition predicates ---
same_dept if {
  input.subject.attrs.dept == input.resource.attrs.dept
}

shift_ok if {
  input.env.time_hour >= 8
  input.env.time_hour <= 18
}

# OR via multiple clauses
sensitive_or_vip if {
  input.resource.attrs.sensitivity == "HIGH"
}
sensitive_or_vip if {
  input.resource.attrs.is_vip == true
}

# Map condition name -> predicate (use boolean function, not partial set)
cond_holds(name) if {
  name == "same_dept"
  same_dept
}
cond_holds(name) if {
  name == "shift_ok"
  shift_ok
}
cond_holds(name) if {
  name == "sensitive_or_vip"
  sensitive_or_vip
}

# Require ALL conditions in the list to hold (treat missing as empty)
all_conds_hold(names) if {
  is_array(names)
  every n in names { cond_holds(n) }
}
all_conds_hold(names) if {
  not names
}

# Helper: subject attrs contain probe k/v
attr_contains(hay, probe) if {
  every k, v in probe { hay[k] == v }
}

# --- Prohibitions: deny overrides ---
deny if {
  p := input.edges.prohib[_]
  input.action in p.ops
  attr_contains(input.subject.attrs, {"role": p.ua})
  input.resource.kind == p.oa
  all_conds_hold(p.conds)
}

# --- Permissions: at least one satisfied edge ---
permit if {
  e := input.edges.perm[_]
  input.action in e.ops
  attr_contains(input.subject.attrs, {"role": e.ua})
  input.resource.kind == e.oa
  all_conds_hold(e.conds)
}

# --- Final decision ---
allow if {
  not deny
  permit
}

# --- Reason ---
reason := "prohibition matched" if { deny }
reason := "no permission edge satisfied" if { not permit }
reason := "allowed" if { allow }

# --- Obligations (only when allowed) ---
obligations := [
  {
    "type": "log",
    "phase": "post",
    "level": "INFO",
    "message": sprintf("user %s read %s", [input.subject.id, input.resource.id])
  }
] if {
  allow
}

# --- Result object ---
result := {
  "allow": allow,
  "reason": reason,
  "obligations": obligations,
  "attributes": {
    "row_filter": sprintf("dept = '%s'", [input.subject.attrs.dept])
  }
}
