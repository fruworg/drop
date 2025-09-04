#!/bin/zsh

# Simple chunked upload client
# Usage: ./upload_client.zsh [file_path] [options]

set -e

SERVER_URL="http://localhost:3000"
CHUNK_SIZE=4194304  # 4MB chunks

show_usage() {
    cat << EOF
Simple Upload Client

Usage: $0 [file_path] [options]

Options:
    -u, --url URL        Custom server URL (default: $SERVER_URL)
    -c, --chunked        Force chunked upload
    -h, --help           Show this help

EOF
}

format_bytes() {
    local bytes="$1"
    if [[ $bytes -ge 1073741824 ]]; then
        printf "%.1f GB" $(echo "$bytes / 1073741824" | bc -l)
    elif [[ $bytes -ge 1048576 ]]; then
        printf "%.1f MB" $(echo "$bytes / 1048576" | bc -l)
    elif [[ $bytes -ge 1024 ]]; then
        printf "%.1f KB" $(echo "$bytes / 1024" | bc -l)
    else
        printf "%d bytes" $bytes
    fi
}

upload_regular() {
    local file_path="$1"
    local filename=$(basename "$file_path")
    local file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null)
    
    echo "📤 Uploading $filename ($(format_bytes $file_size))..."
    
    local response=$(curl -s -X POST "$SERVER_URL" -F "file=@$file_path" -F "one_time=")
    
    if echo "$response" | grep -q "http"; then
        local file_url=$(echo "$response" | tail -n 1)
        echo "🔗 $file_url"
    else
        echo "❌ Upload failed"
        return 1
    fi
}

upload_chunked() {
    local file_path="$1"
    local filename=$(basename "$file_path")
    local file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null)
    
    echo "📤 Uploading $filename ($(format_bytes $file_size))..."
    
    # Initialize upload
    local init_response=$(curl -s -X POST "$SERVER_URL/upload/init" \
        -F "filename=$filename" \
        -F "size=$file_size" \
        -F "chunk_size=$CHUNK_SIZE")
    
    if echo "$init_response" | grep -q '"error"'; then
        local error_msg=$(echo "$init_response" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
        echo "❌ Upload failed: $error_msg"
        return 1
    fi
    
    local upload_id=$(echo "$init_response" | grep -o '"upload_id":"[^"]*"' | cut -d'"' -f4)
    local total_chunks=$(echo "$init_response" | grep -o '"total_chunks":[0-9]*' | cut -d':' -f2)
    
    echo "🚀 Starting chunked upload: $total_chunks chunks of $(format_bytes $CHUNK_SIZE) each"
    
    # Upload chunks with simple progress
    for ((i=0; i<total_chunks; i++)); do
        local chunk_file="/tmp/chunk_${upload_id}_${i}"
        dd if="$file_path" of="$chunk_file" bs=1M skip=$i count=1 2>/dev/null
        
        local chunk_response=$(curl -s -X POST "$SERVER_URL/upload/chunk/$upload_id/$i" -F "chunk=@$chunk_file")
        rm -f "$chunk_file"
        
        if echo "$chunk_response" | grep -q '"progress"'; then
            local progress=$(echo "$chunk_response" | grep -o '"progress":[0-9]*' | cut -d':' -f2)
            
            # Show simple progress
            printf "\r📤 Progress: %d%% (%d/%d chunks)" $progress $((i+1)) $total_chunks
            
            if [[ "$progress" == "100" ]]; then
                local file_url=$(echo "$chunk_response" | grep -o '"file_url":"[^"]*"' | cut -d'"' -f4)
                echo ""  # New line after progress
                echo "🔗 $file_url"
                return 0
            fi
        else
            echo ""  # New line after progress
            echo "❌ Chunk $((i+1)) failed"
            return 1
        fi
    done
}

main() {
    local file_path=""
    local force_chunked=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help) show_usage; exit 0 ;;
            -u|--url) SERVER_URL="$2"; shift 2 ;;
            -c|--chunked) force_chunked=true; shift ;;
            -*)
                echo "❌ Unknown option: $1"
                show_usage; exit 1
                ;;
            *)
                if [[ -z "$file_path" ]]; then
                    file_path="$1"
                else
                    echo "❌ Multiple files specified"
                    exit 1
                fi
                shift
                ;;
        esac
    done
    
    if [[ -z "$file_path" ]]; then
        echo "❌ No file specified"
        show_usage; exit 1
    fi
    
    if [[ ! -f "$file_path" ]]; then
        echo "❌ File not found: $file_path"
        exit 1
    fi
    
    # Check server
    if ! curl -s "$SERVER_URL" > /dev/null; then
        echo "❌ Server not responding at $SERVER_URL"
        exit 1
    fi
    
    # Determine upload method
    local file_size=$(stat -f%z "$file_path" 2>/dev/null || stat -c%s "$file_path" 2>/dev/null)
    local use_chunked=$force_chunked
    
    if [[ $file_size -gt 10485760 ]]; then  # 10MB threshold
        use_chunked=true
    fi
    
    if [[ "$use_chunked" == "true" ]]; then
        upload_chunked "$file_path"
    else
        upload_regular "$file_path"
    fi
}

main "$@"
