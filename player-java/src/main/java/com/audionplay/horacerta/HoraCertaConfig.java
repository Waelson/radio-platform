package com.audionplay.horacerta;

import java.util.ArrayList;
import java.util.List;
import java.util.prefs.Preferences;

/**
 * Configuração da Hora Certa — persiste via java.util.prefs.Preferences.
 *
 * Compatível com o HoraCertaConfig do playout Go:
 *   Hora:    HRS{HH}.mp3  (ex.: HRS14.mp3)
 *   Minuto:  MIN{MM}.mp3  (ex.: MIN35.mp3)
 *   MIN00 é opcional; se ausente e minuto == 0, toca só o arquivo de hora.
 */
public record HoraCertaConfig(
    boolean       enabled,
    String        hoursDir,
    String        minutesDir,
    String        hourPattern,      // "HRS{HH}.mp3"
    String        minutePattern,    // "MIN{MM}.mp3"
    double        gainDb,
    List<Integer> firesAtMinutes    // quais minutos da hora disparam (ex.: [0] = topo da hora)
) {

    private static final String PREF_NODE    = "com.audionplay.horacerta";
    private static final String KEY_ENABLED  = "enabled";
    private static final String KEY_HRS_DIR  = "hours_dir";
    private static final String KEY_MIN_DIR  = "minutes_dir";
    private static final String KEY_HOUR_PAT = "hour_pattern";
    private static final String KEY_MIN_PAT  = "minute_pattern";
    private static final String KEY_GAIN_DB  = "gain_db";
    private static final String KEY_FIRES_AT = "fires_at_minutes";

    public static HoraCertaConfig defaults() {
        return new HoraCertaConfig(
            true,
            "/Users/waelson/Documents/medias/hora_certa/hours_dir",
            "/Users/waelson/Documents/medias/hora_certa/minutes_dir",
            "HRS{HH}.mp3",
            "MIN{MM}.mp3",
            0.0,
            List.of(0)
        );
    }

    public static HoraCertaConfig load() {
        Preferences prefs   = Preferences.userRoot().node(PREF_NODE);
        boolean   enabled   = prefs.getBoolean(KEY_ENABLED, false);
        String    hoursDir  = prefs.get(KEY_HRS_DIR,  "");
        String    minDir    = prefs.get(KEY_MIN_DIR,  "");
        String    hourPat   = prefs.get(KEY_HOUR_PAT, "HRS{HH}.mp3");
        String    minPat    = prefs.get(KEY_MIN_PAT,  "MIN{MM}.mp3");
        double    gainDb    = prefs.getDouble(KEY_GAIN_DB, 0.0);
        String    firesStr  = prefs.get(KEY_FIRES_AT, "0");

        List<Integer> fires = new ArrayList<>();
        for (String s : firesStr.split(",")) {
            try { fires.add(Integer.parseInt(s.trim())); } catch (NumberFormatException ignored) {}
        }
        if (fires.isEmpty()) fires.add(0);

        return new HoraCertaConfig(enabled, hoursDir, minDir, hourPat, minPat, gainDb, fires);
    }

    public void save() {
        Preferences prefs = Preferences.userRoot().node(PREF_NODE);
        prefs.putBoolean(KEY_ENABLED, enabled);
        prefs.put(KEY_HRS_DIR,  hoursDir  != null ? hoursDir  : "");
        prefs.put(KEY_MIN_DIR,  minutesDir != null ? minutesDir : "");
        prefs.put(KEY_HOUR_PAT, hourPattern  != null && !hourPattern.isEmpty()  ? hourPattern  : "HRS{HH}.mp3");
        prefs.put(KEY_MIN_PAT,  minutePattern != null && !minutePattern.isEmpty() ? minutePattern : "MIN{MM}.mp3");
        prefs.putDouble(KEY_GAIN_DB, gainDb);
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < firesAtMinutes.size(); i++) {
            if (i > 0) sb.append(",");
            sb.append(firesAtMinutes.get(i));
        }
        prefs.put(KEY_FIRES_AT, sb.isEmpty() ? "0" : sb.toString());
        try { prefs.flush(); } catch (Exception ignored) {}
    }
}
