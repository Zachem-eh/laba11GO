package main

import (
	"context"
	"database/sql"
	"fmt"
)

// CategoryRevenue представляет результат агрегации по категориям
type CategoryRevenue struct {
	CategoryName    string
	TotalSalesCount int     // COUNT
	TotalRevenue    float64 // SUM
	AvgRevenue      float64 // AVG
}

func GetRevenueByCategory(ctx context.Context, db *sql.DB) ([]CategoryRevenue, error) {
	query := `
		SELECT 
			c.name AS category_name,
			COUNT(s.id) AS total_sales_count,
			SUM(s.total_price) AS total_revenue,
			AVG(s.total_price) AS avg_revenue_per_sale
		FROM sales s
		JOIN products p ON s.product_id = p.id
		JOIN categories c ON p.category_id = c.id
		GROUP BY c.id, c.name
		ORDER BY total_revenue DESC
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса выручки по категориям: %w", err)
	}
	defer rows.Close()

	var results []CategoryRevenue
	for rows.Next() {
		var cr CategoryRevenue

		var avgRev sql.NullFloat64
		err := rows.Scan(&cr.CategoryName, &cr.TotalSalesCount, &cr.TotalRevenue, &avgRev)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки: %w", err)
		}

		if avgRev.Valid {
			cr.AvgRevenue = avgRev.Float64
		}
		results = append(results, cr)

	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по строкам: %w", err)
	}
	return results, nil
}

// TopProduct представляет топ продаваемых товаров
type TopProduct struct {
	ProductName   string
	CategoryName  string
	TotalQuantity int
	TotalRevenue  float64
}

func GetTopProducts(ctx context.Context, db *sql.DB, minPrice float64, limit int) ([]TopProduct, error) {
	query := `
		SELECT 
			p.name AS product_name,
			c.name AS category_name,
			SUM(s.quantity) AS total_quantity_sold,
			SUM(s.total_price) AS total_revenue
		FROM sales s
		JOIN products p ON s.product_id = p.id
		JOIN categories c ON p.category_id = c.id
		WHERE p.price >= ? -- Фильтрация по цене
		GROUP BY p.id, p.name, c.name
		ORDER BY total_revenue DESC
		LIMIT ? -- Ограничение выборки (топ-N)
	`
	rows, err := db.QueryContext(ctx, query, minPrice, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса топ товаров: %w", err)
	}
	defer rows.Close()

	var results []TopProduct
	for rows.Next() {
		var tp TopProduct
		err := rows.Scan(&tp.ProductName, &tp.CategoryName, &tp.TotalQuantity, &tp.TotalRevenue)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки топ товара: %w", err)
		}
		results = append(results, tp)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по строкам топ товаров: %w", err)
	}
	return results, nil
}
