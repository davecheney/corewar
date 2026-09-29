;redcode-94
;name Mice
;author Chip Wendell
; Copies itself, splits into the copy, and moves on to copy again.
ptr     DAT     #0
start   MOV     #12, ptr
loop    MOV     @ptr, <copy
        DJN     loop, ptr
        SPL     @copy
        ADD     #653, copy
        JMZ     start, ptr
copy    DAT     #833
        END     start
