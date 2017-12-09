#!/bin/bash

_gobgp_global_rib_add()
{
    last_command="gobgp_global_rib_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_rib_del()
{
    last_command="gobgp_global_rib_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_rib()
{
    last_command="gobgp_global_rib"
    commands=()
    commands+=("add")
    commands+=("del")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_in_add()
{
    last_command="gobgp_global_policy_in_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_in_del()
{
    last_command="gobgp_global_policy_in_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_in_set()
{
    last_command="gobgp_global_policy_in_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_in()
{
    last_command="gobgp_global_policy_in"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_import_add()
{
    last_command="gobgp_global_policy_import_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_import_del()
{
    last_command="gobgp_global_policy_import_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_import_set()
{
    last_command="gobgp_global_policy_import_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_import()
{
    last_command="gobgp_global_policy_import"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_export_add()
{
    last_command="gobgp_global_policy_export_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_export_del()
{
    last_command="gobgp_global_policy_export_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_export_set()
{
    last_command="gobgp_global_policy_export_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy_export()
{
    last_command="gobgp_global_policy_export"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global_policy()
{
    last_command="gobgp_global_policy"
    commands=()
    commands+=("in")
    commands+=("import")
    commands+=("export")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_global()
{
    last_command="gobgp_global"
    commands=()
    commands+=("rib")
    commands+=("policy")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}


_gobgp_neighbor()
{
    last_command="gobgp_neighbor"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--transport=")
    two_word_flags+=("-t")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_neighbor
}

_gobgp_vrf_add()
{
    last_command="gobgp_vrf_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_vrf_del()
{
    last_command="gobgp_vrf_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_vrf
}

_gobgp_vrf()
{
    last_command="gobgp_vrf"
    commands=()
    commands+=("add")
    commands+=("del")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_vrf
}

_gobgp_policy_prefix_add()
{
    last_command="gobgp_policy_prefix_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_prefix_del()
{
    last_command="gobgp_policy_prefix_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "prefix"
}

_gobgp_policy_prefix_set()
{
    last_command="gobgp_policy_prefix_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "prefix"
}

_gobgp_policy_prefix()
{
    last_command="gobgp_policy_prefix"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_neighbor_add()
{
    last_command="gobgp_policy_neighbor_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_neighbor_del()
{
    last_command="gobgp_policy_neighbor_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "neighbor"
}

_gobgp_policy_neighbor_set()
{
    last_command="gobgp_policy_neighbor_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "neighbor"
}

_gobgp_policy_neighbor()
{
    last_command="gobgp_policy_neighbor"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_as-path_add()
{
    last_command="gobgp_policy_as-path_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_as-path_del()
{
    last_command="gobgp_policy_as-path_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "as-path"
}

_gobgp_policy_as-path_set()
{
    last_command="gobgp_policy_as-path_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "as-path"
}

_gobgp_policy_as-path()
{
    last_command="gobgp_policy_as-path"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_community_add()
{
    last_command="gobgp_policy_community_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_community_del()
{
    last_command="gobgp_policy_community_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "community"
}

_gobgp_policy_community_set()
{
    last_command="gobgp_policy_community_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "community"
}

_gobgp_policy_community()
{
    last_command="gobgp_policy_community"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_ext-community_add()
{
    last_command="gobgp_policy_ext-community_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_ext-community_del()
{
    last_command="gobgp_policy_ext-community_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "ext-community"
}

_gobgp_policy_ext-community_set()
{
    last_command="gobgp_policy_ext-community_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy "ext-community"
}

_gobgp_policy_ext-community()
{
    last_command="gobgp_policy_ext-community"
    commands=()
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_statement_add()
{
    last_command="gobgp_policy_statement_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_statement_del()
{
    last_command="gobgp_policy_statement_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_statement
}

_gobgp_policy_statement()
{
    last_command="gobgp_policy_statement"
    commands=()
    commands+=("add")
    commands+=("del")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_statement
}

_gobgp_policy_add()
{
    last_command="gobgp_policy_add"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_policy_del()
{
    last_command="gobgp_policy_del"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy ""
}

_gobgp_policy_set()
{
    last_command="gobgp_policy_set"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_policy ""
}

_gobgp_policy()
{
    last_command="gobgp_policy"
    commands=()
    commands+=("prefix")
    commands+=("neighbor")
    commands+=("as-path")
    commands+=("community")
    commands+=("ext-community")
    commands+=("statement")
    commands+=("add")
    commands+=("del")
    commands+=("set")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_monitor_global_rib()
{
    last_command="gobgp_monitor_global_rib"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_monitor_global()
{
    last_command="gobgp_monitor_global"
    commands=()
    commands+=("rib")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_monitor_neighbor()
{
    last_command="gobgp_monitor_neighbor"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_neighbor
}

_gobgp_monitor()
{
    last_command="gobgp_monitor"
    commands=()
    commands+=("global")
    commands+=("neighbor")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}


_gobgp_mrt_dump_rib_global()
{
    last_command="gobgp_mrt_dump_rib_global"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--format=")
    two_word_flags+=("-f")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--outdir=")
    two_word_flags+=("-o")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_dump_rib_neighbor()
{
    last_command="gobgp_mrt_dump_rib_neighbor"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--format=")
    two_word_flags+=("-f")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--outdir=")
    two_word_flags+=("-o")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
    __gobgp_q_neighbor
}

_gobgp_mrt_dump_rib()
{
    last_command="gobgp_mrt_dump_rib"
    commands=()
    commands+=("global")
    commands+=("neighbor")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--format=")
    two_word_flags+=("-f")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--outdir=")
    two_word_flags+=("-o")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_dump()
{
    last_command="gobgp_mrt_dump"
    commands=()
    commands+=("rib")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--format=")
    two_word_flags+=("-f")
    flags+=("--outdir=")
    two_word_flags+=("-o")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_inject_global()
{
    last_command="gobgp_mrt_inject_global"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_inject()
{
    last_command="gobgp_mrt_inject"
    commands=()
    commands+=("global")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_update_enable()
{
    last_command="gobgp_mrt_update_enable"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_update_disable()
{
    last_command="gobgp_mrt_update_disable"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_update_reset()
{
    last_command="gobgp_mrt_update_reset"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_update_rotate()
{
    last_command="gobgp_mrt_update_rotate"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt_update()
{
    last_command="gobgp_mrt_update"
    commands=()
    commands+=("enable")
    commands+=("disable")
    commands+=("reset")
    commands+=("rotate")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_mrt()
{
    last_command="gobgp_mrt"
    commands=()
    commands+=("dump")
    commands+=("inject")
    commands+=("update")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_rpki_enable()
{
    last_command="gobgp_rpki_enable"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_rpki_server()
{
    last_command="gobgp_rpki_server"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_rpki_table()
{
    last_command="gobgp_rpki_table"
    commands=()

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--address-family=")
    two_word_flags+=("-a")
    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp_rpki()
{
    last_command="gobgp_rpki"
    commands=()
    commands+=("enable")
    commands+=("server")
    commands+=("table")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}

_gobgp()
{
    url=""
    port=""
    q_type=""
    last_command="gobgp"
    commands=()
    commands+=("global")
    commands+=("neighbor")
    commands+=("vrf")
    commands+=("policy")
    commands+=("monitor")
    commands+=("mrt")
    commands+=("rpki")

    flags=()
    two_word_flags=()
    flags_with_completion=()
    flags_completion=()

    flags+=("--bash-cmpl-file=")
    flags+=("--debug")
    flags+=("-d")
    flags+=("--gen-cmpl")
    flags+=("-c")
    flags+=("--host=")
    two_word_flags+=("-u")
    flags+=("--json")
    flags+=("-j")
    flags+=("--port=")
    two_word_flags+=("-p")
    flags+=("--quiet")
    flags+=("-q")

    must_have_one_flag=()
    must_have_one_noun=()
}