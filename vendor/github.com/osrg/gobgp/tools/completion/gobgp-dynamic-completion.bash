#!/bin/bash

__gobgp_q()
{
    gobgp 2>/dev/null "$@"
}

# Get bgp neighbors use gobgp command.
__gobgp_q_neighbor()
{
    local neighbors=( $(__gobgp_q $url $port --quiet neighbor) )
    case "${neighbors[*]}" in
        "grpc: timed out"* | "rpc error:"* )
            req_faild="True"
            return
        ;;
    esac
    for n in ${neighbors[*]}; do
        commands+=($n)
    done
    searched="True"
}

# Get gobgp configration of vrfs use gobgp command.
__gobgp_q_vrf()
{
    local vrfs=( $(__gobgp_q $url $port --quiet vrf) )
    case "${vrfs[*]}" in
        "grpc: timed out"* | "rpc error:"* )
            req_faild="True"
            return
        ;;
    esac
    for n in ${vrfs[*]}; do
        commands+=($n)
    done
    searched="True"
}

# Get gobgp configration of policies use gobgp command.
__gobgp_q_policy()
{
    local parg=$1
    local policies=( $(__gobgp_q $url $port --quiet policy $parg) )
    case "${policies[*]}" in
        "grpc: timed out"* | "rpc error:"* )
            req_faild="True"
            return
        ;;
    esac
    for ps in ${policies[*]}; do
        commands+=($ps)
    done
    searched="True"
}

# Get gobgp configration of policiy statements use gobgp command.
__gobgp_q_statement()
{
    local statements=( $(__gobgp_q $url $port --quiet policy statement ) )
    case "${statements[*]}" in
        "grpc: timed out"* | "rpc error:"* )
            req_faild="True"
            return
        ;;
    esac
    for sts in ${statements[*]}; do
        commands+=($sts)
    done
    searched="True"
}

# Handler for controlling obtained when the dynamic complement.
# This function checks the last command to control the next operation.
__handle_gobgp_command()
{
    if [[ ${searched} == "True" ]]; then
        case "${last_command}" in
            # Control after dynamic complement of bgp neighbor command
            gobgp_neighbor )
                next_command="_${last_command}_addr"
            ;;

            # Control after dynamic complement of bgp policy command
            gobgp_policy_prefix_* | gobgp_policy_neighbor_* | gobgp_policy_as-path_* | gobgp_policy_community_* | gobgp_policy_ext-community_* )
                next_command="__gobgp_null"
            ;;
            gobgp_policy_del | gobgp_policy_set )
                next_command="__gobgp_null"
            ;;
            gobgp_policy_statement )
                if [[ ${words[c]} == "del" || ${words[c]} == "add" ]]; then
                    return
                fi
                next_command="_gobgp_policy_statement_sname"
            ;;
            gobgp_policy_statement_del )
                next_command="__gobgp_null"
            ;;
            *_condition_prefix | *_condition_neighbor | *_condition_as-path | *_condition_community  | *_ext-condition_community )
                next_command="__gobgp_null"
            ;;

            # Control after dynamic complement of bgp vrf command
            gobgp_vrf )
                if [[ ${words[c]} == "del" || ${words[c]} == "add" ]]; then
                    return
                fi
                next_command="_global_vrf_vname"
            ;;
            gobgp_vrf_del )
                next_command="__gobgp_null"
            ;;

            # Control after dynamic complement of bgp mrt command
            gobgp_mrt_dump_rib_neighbor )
                next_command="__gobgp_null"
            ;;
            gobgp_monitor_neighbor )
                next_command="__gobgp_null"
            ;;
        esac
        through="True"
    fi
}

__gobgp_null()
{
    last_command="gobgp_null"
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


_gobgp_neighbor_addr_local()
{
    last_command="gobgp_neighbor_addr_local"
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

_gobgp_neighbor_addr_adj-in()
{
    last_command="gobgp_neighbor_addr_adj-in"
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

_gobgp_neighbor_addr_adj-out()
{
    last_command="gobgp_neighbor_addr_adj-out"
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

_gobgp_neighbor_addr_reset()
{
    last_command="gobgp_neighbor_addr_reset"
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

_gobgp_neighbor_addr_softreset()
{
    last_command="gobgp_neighbor_addr_softreset"
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

_gobgp_neighbor_addr_softresetin()
{
    last_command="gobgp_neighbor_addr_softresetin"
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

_gobgp_neighbor_addr_softresetout()
{
    last_command="gobgp_neighbor_addr_softresetout"
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

_gobgp_neighbor_addr_shutdown()
{
    last_command="gobgp_neighbor_addr_shutdown"
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

_gobgp_neighbor_addr_enable()
{
    last_command="gobgp_neighbor_addr_enable"
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

_gobgp_neighbor_addr_disable()
{
    last_command="gobgp_neighbor_addr_disable"
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

_gobgp_neighbor_addr_policy_in_add()
{
    last_command="gobgp_neighbor_addr_policy_in_add"
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

_gobgp_neighbor_addr_policy_in_del()
{
    last_command="gobgp_neighbor_addr_policy_in_del"
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

_gobgp_neighbor_addr_policy_in_set()
{
    last_command="gobgp_neighbor_addr_policy_in_set"
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

_gobgp_neighbor_addr_policy_in()
{
    last_command="gobgp_neighbor_addr_policy_in"
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

_gobgp_neighbor_addr_policy_import_add()
{
    last_command="gobgp_neighbor_addr_policy_import_add"
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

_gobgp_neighbor_addr_policy_import_del()
{
    last_command="gobgp_neighbor_addr_policy_import_del"
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

_gobgp_neighbor_addr_policy_import_set()
{
    last_command="gobgp_neighbor_addr_policy_import_set"
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

_gobgp_neighbor_addr_policy_import()
{
    last_command="gobgp_neighbor_addr_policy_import"
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

_gobgp_neighbor_addr_policy_export_add()
{
    last_command="gobgp_neighbor_addr_policy_export_add"
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

_gobgp_neighbor_addr_policy_export_del()
{
    last_command="gobgp_neighbor_addr_policy_export_del"
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

_gobgp_neighbor_addr_policy_export_set()
{
    last_command="gobgp_neighbor_addr_policy_export_set"
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

_gobgp_neighbor_addr_policy_export()
{
    last_command="gobgp_neighbor_addr_policy_export"
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

_gobgp_neighbor_addr_policy()
{
    last_command="gobgp_neighbor_addr_policy"
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

_gobgp_neighbor_addr()
{
    last_command="gobgp_neighbor_addr"
    commands=()
    commands+=("local")
    commands+=("adj-in")
    commands+=("adj-out")
    commands+=("reset")
    commands+=("softreset")
    commands+=("softresetin")
    commands+=("softresetout")
    commands+=("shutdown")
    commands+=("enable")
    commands+=("disable")
    commands+=("policy")

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

_global_vrf_vname_rib_del()
{
    last_command="global_vrf_vname_rib_del"
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

_global_vrf_vname_rib()
{
    last_command="global_vrf_vname_rib"
    commands=()
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
}

_global_vrf_vname()
{
    last_command="global_vrf_vname"
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

_gobgp_policy_statement_sname_ope_condition_prefix()
{
    last_command="gobgp_policy_statement_sname_ope_condition_prefix"
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

_gobgp_policy_statement_sname_ope_condition_neighbor()
{
    last_command="gobgp_policy_statement_sname_ope_condition_neighbor"
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

_gobgp_policy_statement_sname_ope_condition_as-path()
{
    last_command="gobgp_policy_statement_sname_ope_condition_as-path"
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

_gobgp_policy_statement_sname_ope_condition_community()
{
    last_command="gobgp_policy_statement_sname_ope_condition_community"
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

_gobgp_policy_statement_sname_ope_condition_ext-community()
{
    last_command="gobgp_policy_statement_sname_ope_condition_ext-community"
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

_gobgp_policy_statement_sname_ope_condition_as-path-length()
{
    last_command="gobgp_policy_statement_sname_ope_condition_as-path-length"
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

_gobgp_policy_statement_sname_ope_condition_rpki_valid()
{
    last_command="gobgp_policy_statement_sname_ope_condition_rpki_valid"
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

_gobgp_policy_statement_sname_ope_condition_rpki_invalid()
{
    last_command="gobgp_policy_statement_sname_ope_condition_rpki_invalid"
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

_gobgp_policy_statement_sname_ope_condition_rpki_not-found()
{
    last_command="gobgp_policy_statement_sname_ope_condition_rpki_not-found"
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

_gobgp_policy_statement_sname_ope_condition_rpki()
{
    last_command="gobgp_policy_statement_sname_ope_condition_rpki"
    commands=()
    commands+=("valid")
    commands+=("invalid")
    commands+=("not-found")

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


_gobgp_policy_statement_sname_ope_condition()
{
    last_command="gobgp_policy_statement_sname_ope_condition"
    commands=()
    commands+=("prefix")
    commands+=("neighbor")
    commands+=("as-path")
    commands+=("community")
    commands+=("ext-community")
    commands+=("as-path-length")
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

_gobgp_policy_statement_sname_ope_action_reject()
{
    last_command="gobgp_policy_statement_sname_ope_action_reject"
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

_gobgp_policy_statement_sname_ope_action_accept()
{
    last_command="gobgp_policy_statement_sname_ope_action_accept"
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

_gobgp_policy_statement_sname_ope_action_communities_add()
{
    last_command="gobgp_policy_statement_sname_ope_action_communities_add"
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

_gobgp_policy_statement_sname_ope_action_communities_remove()
{
    last_command="gobgp_policy_statement_sname_ope_action_communities_remove"
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

_gobgp_policy_statement_sname_ope_action_communities_replace()
{
    last_command="gobgp_policy_statement_sname_ope_action_communities_replace"
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

_gobgp_policy_statement_sname_ope_action_communities()
{
    last_command="gobgp_policy_statement_sname_ope_action_communities"
    commands=()
    commands+=("add")
    commands+=("remove")
    commands+=("replace")

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

_gobgp_policy_statement_sname_ope_action_community()
{
    _gobgp_policy_statement_sname_ope_action_communities
}

_gobgp_policy_statement_sname_ope_action_ext-community()
{
    _gobgp_policy_statement_sname_ope_action_communities
}

_gobgp_policy_statement_sname_ope_action_med_add()
{
    last_command="gobgp_policy_statement_sname_ope_action_med_add"
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

_gobgp_policy_statement_sname_ope_action_med_sub()
{
    last_command="gobgp_policy_statement_sname_ope_action_med_sub"
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

_gobgp_policy_statement_sname_ope_action_med_set()
{
    last_command="gobgp_policy_statement_sname_ope_action_med_set"
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

_gobgp_policy_statement_sname_ope_action_med()
{
    last_command="gobgp_policy_statement_sname_ope_action_med"
    commands=()
    commands+=("add")
    commands+=("sub")
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

_gobgp_policy_statement_sname_ope_action_as-prepend()
{
    last_command="gobgp_policy_statement_sname_ope_action_as-prepend"
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

_gobgp_policy_statement_sname_ope_action()
{
    last_command="gobgp_policy_statement_sname_ope_action"
    commands=()
    commands+=("reject")
    commands+=("accept")
    commands+=("community")
    commands+=("ext-community")
    commands+=("med")
    commands+=("as-prepend")

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

_gobgp_policy_statement_sname_ope()
{
    last_command="gobgp_policy_statement_sname_ope"
    commands=()
    commands+=("condition")
    commands+=("action")

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

_gobgp_policy_statement_sname_add()
{
    _gobgp_policy_statement_sname_ope
}

_gobgp_policy_statement_sname_del()
{
    _gobgp_policy_statement_sname_ope
}

_gobgp_policy_statement_sname_set()
{
    _gobgp_policy_statement_sname_ope
}

_gobgp_policy_statement_sname()
{
    last_command="gobgp_policy_statement_sname"
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