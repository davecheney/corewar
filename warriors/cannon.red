;redcode-94
;name Cannon
;author corewar demo
; An eight-cell dwarf: its bomb stream remains in the empty eighth lane.
        ORG     loop
loop    ADD.AB  #8, bomb
        MOV.I   bomb, @bomb
        JMP     loop
        NOP
        NOP
        NOP
        NOP
bomb    DAT     #0, #0
