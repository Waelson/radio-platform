package com.audionplay.db.entity;

/** Uma célula da grade 24×7 (tabela {@code clock_schedule}). */
public record ClockScheduleEntry(
    int    weekday,    // 0=Dom … 6=Sáb
    int    hour,       // 0-23
    String clockId,
    String clockName   // resolvido via JOIN
) {}
