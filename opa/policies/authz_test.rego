package authz_test

import data.authz

# --- ALLOW: doctor, same dept, within shift; ALL conds hold ---
test_allow_doctor_same_dept_and_shift if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_1","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow == true
  res.reason == "allowed"
  count(res.obligations) > 0
}

# --- DENY: prohibition overrides via sensitive_or_vip ---
test_deny_prohibition_overrides_sensitive if {
  req := {
    "subject": {"id":"u2","attrs":{"role":"intern","dept":"cardiology"}},
    "resource":{"id":"rec_2","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"HIGH","is_vip":false}},
    "action":"read",
    "env":{"time_hour":11},
    "edges":{
      "perm":[{"ua":"intern","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[{"ua":"intern","ops":["read"],"oa":"patient_record","conds":["sensitive_or_vip"]}]
    }
  }
  res := authz.result with input as req
  res.allow == false
  res.reason == "prohibition matched"
  count(res.obligations) == 0
}

# --- DENY: ALL conditions must hold (shift not ok) ---
test_deny_when_one_required_condition_fails if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_3","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":22},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow == false
  res.reason == "no permission edge satisfied"
}

# --- ALLOW: permit edge with conds omitted (treated as empty) ---
test_allow_when_conditions_omitted_treated_empty if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_4","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record"}],  # conds omitted
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow == true
  res.reason == "allowed"
}

# --- ALLOW: explicit empty conds [] ---
test_allow_when_conditions_empty_array if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_5","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":[]}],
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow == true
  res.reason == "allowed"
}

# --- DENY: no matching permit edges at all ---
test_deny_when_no_permit_edges_match if {
  req := {
    "subject": {"id":"u9","attrs":{"role":"nurse","dept":"cardiology"}},
    "resource":{"id":"rec_6","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept"]}],
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow == false
  res.reason == "no permission edge satisfied"
}

# --- ALLOW: no prohibition hit (LOW sensitivity, VIP false) ---
test_allow_when_no_prohibition_condition_matches if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_7","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["sensitive_or_vip"]}]
    }
  }
  res := authz.result with input as req
  res.allow == true
  res.reason == "allowed"
}

# --- DENY: VIP path triggers prohibition (OR within predicate) ---
test_deny_when_vip_triggers_prohibition if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"cardiology"}},
    "resource":{"id":"rec_8","kind":"patient_record","attrs":{"dept":"cardiology","sensitivity":"LOW","is_vip":true}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["sensitive_or_vip"]}]
    }
  }
  res := authz.result with input as req
  res.allow == false
  res.reason == "prohibition matched"
}

# --- ATTRIBUTES payload sanity ---
test_attributes_row_filter_in_result if {
  req := {
    "subject": {"id":"u1","attrs":{"role":"doctor","dept":"oncology"}},
    "resource":{"id":"rec_9","kind":"patient_record","attrs":{"dept":"oncology","sensitivity":"LOW","is_vip":false}},
    "action":"read",
    "env":{"time_hour":10},
    "edges":{
      "perm":[{"ua":"doctor","ops":["read"],"oa":"patient_record","conds":["same_dept","shift_ok"]}],
      "prohib":[]
    }
  }
  res := authz.result with input as req
  res.allow
  startswith(res.attributes.row_filter, "dept = 'oncology'")
}
