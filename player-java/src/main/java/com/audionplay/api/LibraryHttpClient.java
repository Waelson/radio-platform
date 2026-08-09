package com.audionplay.api;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;

/**
 * Cliente HTTP mínimo para o Library Service.
 *
 * Usa java.net.http (Java 11+) sem dependências externas.
 * Autenticação via JWT — obtido em /v1/auth/login.
 *
 * JSON parsing manual para os dois únicos endpoints usados:
 *   POST /v1/auth/login   → extrai "token"
 *   POST /v1/schedule/generate → extrai lista de items e warnings
 */
public class LibraryHttpClient {

    private final String baseUrl;
    private String token;

    private final HttpClient http = HttpClient.newBuilder()
        .connectTimeout(Duration.ofSeconds(5))
        .build();

    public LibraryHttpClient(String baseUrl) {
        this.baseUrl = baseUrl.replaceAll("/+$", "");
    }

    /** Autentica e armazena o JWT. @return true se bem-sucedido. */
    public boolean login(String email, String password) {
        try {
            String body = "{\"email\":\"" + esc(email) + "\",\"password\":\"" + esc(password) + "\"}";
            HttpRequest req = HttpRequest.newBuilder()
                .uri(URI.create(baseUrl + "/v1/auth/login"))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body))
                .timeout(Duration.ofSeconds(10))
                .build();
            HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
            if (resp.statusCode() == 200) {
                token = extractString(resp.body(), "token");
                return token != null && !token.isBlank();
            }
            return false;
        } catch (Exception e) {
            return false;
        }
    }

    public boolean isAuthenticated() { return token != null && !token.isBlank(); }

    /**
     * Chama POST /v1/schedule/generate e retorna o resultado parseado.
     * @param from  ISO-8601 sem timezone, ex: "2024-01-15T08:00:00"
     * @param hours número de horas a gerar
     */
    public GenerateResult generateSchedule(String from, int hours) throws Exception {
        String body = "{\"from\":\"" + from + "\",\"hours\":" + hours + "}";
        HttpRequest req = HttpRequest.newBuilder()
            .uri(URI.create(baseUrl + "/v1/schedule/generate"))
            .header("Content-Type", "application/json")
            .header("Authorization", "Bearer " + token)
            .POST(HttpRequest.BodyPublishers.ofString(body))
            .timeout(Duration.ofSeconds(30))
            .build();
        HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
        return parseGenerateResult(resp.body());
    }

    // ── Data classes ──────────────────────────────────────────────────────────

    public record TrackData(
        String id, String path, String title, String artist,
        String type, long durationMs
    ) {}

    public record GenerateItem(
        String slotType, int position, String clockName,
        String categoryName, int hour, TrackData track
    ) {}

    public record GenerateResult(
        boolean ok, String message,
        List<GenerateItem> items,
        List<String> warnings
    ) {}

    // ── Manual JSON parsing ───────────────────────────────────────────────────

    /** Extrai o valor de uma chave string simples no JSON. */
    private static String extractString(String json, String key) {
        String needle = "\"" + key + "\"";
        int idx = json.indexOf(needle);
        if (idx < 0) return null;
        int colon = json.indexOf(':', idx + needle.length());
        if (colon < 0) return null;
        int q1 = json.indexOf('"', colon + 1);
        if (q1 < 0) return null;
        int q2 = json.indexOf('"', q1 + 1);
        if (q2 < 0) return null;
        return json.substring(q1 + 1, q2);
    }

    private static boolean extractBool(String json, String key) {
        String needle = "\"" + key + "\"";
        int idx = json.indexOf(needle);
        if (idx < 0) return false;
        int colon = json.indexOf(':', idx + needle.length());
        if (colon < 0) return false;
        String rest = json.substring(colon + 1).trim();
        return rest.startsWith("true");
    }

    private static long extractLong(String json, String key) {
        String needle = "\"" + key + "\"";
        int idx = json.indexOf(needle);
        if (idx < 0) return 0;
        int colon = json.indexOf(':', idx + needle.length());
        if (colon < 0) return 0;
        String rest = json.substring(colon + 1).trim();
        StringBuilder sb = new StringBuilder();
        for (char c : rest.toCharArray()) {
            if (Character.isDigit(c) || c == '-') sb.append(c);
            else if (sb.length() > 0) break;
        }
        try { return Long.parseLong(sb.toString()); } catch (NumberFormatException e) { return 0; }
    }

    private static int extractInt(String json, String key) { return (int) extractLong(json, key); }

    /**
     * Extrai um array de objetos delimitados por chaves de nível 1.
     * Funciona para arrays simples (não aninhados em mais de 2 níveis).
     */
    private static List<String> extractObjectArray(String json, String key) {
        String needle = "\"" + key + "\"";
        int idx = json.indexOf(needle);
        if (idx < 0) return List.of();
        int bracket = json.indexOf('[', idx);
        if (bracket < 0) return List.of();
        List<String> objects = new ArrayList<>();
        int depth = 0;
        int start = -1;
        for (int i = bracket + 1; i < json.length(); i++) {
            char c = json.charAt(i);
            if (c == '{') {
                if (depth == 0) start = i;
                depth++;
            } else if (c == '}') {
                depth--;
                if (depth == 0 && start >= 0) {
                    objects.add(json.substring(start, i + 1));
                    start = -1;
                }
            } else if (c == ']' && depth == 0) {
                break;
            }
        }
        return objects;
    }

    /** Extrai strings de um array JSON simples: ["a","b",...]. */
    private static List<String> extractStringArray(String json, String key) {
        String needle = "\"" + key + "\"";
        int idx = json.indexOf(needle);
        if (idx < 0) return List.of();
        int bracket = json.indexOf('[', idx);
        if (bracket < 0) return List.of();
        int end = json.indexOf(']', bracket);
        if (end < 0) return List.of();
        String content = json.substring(bracket + 1, end);
        List<String> result = new ArrayList<>();
        int pos = 0;
        while (pos < content.length()) {
            int q1 = content.indexOf('"', pos);
            if (q1 < 0) break;
            int q2 = content.indexOf('"', q1 + 1);
            if (q2 < 0) break;
            result.add(content.substring(q1 + 1, q2));
            pos = q2 + 1;
        }
        return result;
    }

    private static GenerateResult parseGenerateResult(String json) {
        boolean ok = extractBool(json, "ok");
        String message = extractString(json, "message");

        // Find "data" object
        int dataIdx = json.indexOf("\"data\"");
        String dataJson = dataIdx >= 0 ? json.substring(dataIdx) : json;

        List<String> itemObjs = extractObjectArray(dataJson, "items");
        List<GenerateItem> items = new ArrayList<>();
        for (String obj : itemObjs) {
            String slotType    = extractString(obj, "slot_type");
            int    position    = extractInt(obj, "position");
            String clockName   = extractString(obj, "clock_name");
            String catName     = extractString(obj, "category_name");
            int    hour        = extractInt(obj, "hour");

            // track sub-object
            int trackIdx = obj.indexOf("\"track\"");
            String trackJson = trackIdx >= 0 ? obj.substring(trackIdx) : "{}";
            // find the { after "track":
            int trackBrace = trackJson.indexOf('{', 7);
            String tObj = "{}";
            if (trackBrace >= 0) {
                int depth = 0, end = trackBrace;
                for (int i = trackBrace; i < trackJson.length(); i++) {
                    char c = trackJson.charAt(i);
                    if (c == '{') depth++;
                    else if (c == '}') { depth--; if (depth == 0) { end = i; break; } }
                }
                tObj = trackJson.substring(trackBrace, end + 1);
            }

            TrackData track = new TrackData(
                extractString(tObj, "id"),
                extractString(tObj, "path"),
                coalesce(extractString(tObj, "title"), ""),
                coalesce(extractString(tObj, "artist"), ""),
                coalesce(extractString(tObj, "type"), "MUSIC"),
                extractLong(tObj, "duration_ms")
            );
            items.add(new GenerateItem(slotType, position, clockName, catName, hour, track));
        }

        List<String> warnings = extractStringArray(dataJson, "warnings");
        return new GenerateResult(ok, message, items, warnings);
    }

    private static String coalesce(String s, String def) { return (s == null) ? def : s; }

    private static String esc(String s) {
        return s == null ? "" : s.replace("\\", "\\\\").replace("\"", "\\\"");
    }
}
