;redcode-94
;name Fission
;author corewar demo
; Splits a sparse stream of forward-moving imps.
        ORG     start
start   SPL     spawn
        MOV.I   0, 1
spawn   SPL     2
        JMP     -2
